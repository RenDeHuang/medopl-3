DO $$
BEGIN
  IF to_regclass('control_plane_accounts') IS NULL
    OR to_regclass('control_plane_users') IS NULL
    OR to_regclass('control_plane_organizations') IS NULL
    OR to_regclass('control_plane_memberships') IS NULL
    OR to_regclass('control_plane_sessions') IS NULL
  THEN
    RAISE EXCEPTION 'customer identity truth tables are missing';
  END IF;
END $$;

LOCK TABLE control_plane_accounts, control_plane_users, control_plane_organizations, control_plane_memberships, control_plane_sessions IN ACCESS EXCLUSIVE MODE;

DO $$
BEGIN
  IF EXISTS (
    SELECT 1
    FROM control_plane_accounts accounts
    LEFT JOIN control_plane_users users ON users.account_id = accounts.id
    GROUP BY accounts.id
    HAVING COUNT(users.id) <> 1
  ) OR EXISTS (
    SELECT 1
    FROM control_plane_users users
    LEFT JOIN control_plane_accounts accounts ON accounts.id = users.account_id
    WHERE accounts.id IS NULL
  ) THEN
    RAISE EXCEPTION 'legacy account/user cardinality is not one-to-one';
  END IF;

  IF EXISTS (
    SELECT 1 FROM control_plane_users
    WHERE email IS NULL OR btrim(email) = ''
  ) OR EXISTS (
    SELECT 1 FROM control_plane_users
    GROUP BY lower(btrim(email))
    HAVING COUNT(*) > 1
  ) THEN
    RAISE EXCEPTION 'legacy normalized user email is blank or duplicated';
  END IF;

  IF EXISTS (
    SELECT 1 FROM control_plane_accounts WHERE sub2api_user_id <= 0
  ) OR EXISTS (
    SELECT 1 FROM control_plane_accounts
    GROUP BY sub2api_user_id
    HAVING COUNT(*) > 1
  ) THEN
    RAISE EXCEPTION 'legacy Sub2API mapping is missing or duplicated';
  END IF;

  IF EXISTS (
    SELECT 1
    FROM control_plane_accounts accounts
    LEFT JOIN control_plane_users owners ON owners.id = accounts.owner_user_id
    WHERE accounts.owner_user_id <> ''
      AND (owners.id IS NULL OR owners.account_id <> accounts.id)
  ) OR EXISTS (
    SELECT 1 FROM control_plane_accounts
    WHERE owner_user_id <> ''
    GROUP BY owner_user_id
    HAVING COUNT(*) > 1
  ) THEN
    RAISE EXCEPTION 'legacy account owner is cross-account or missing';
  END IF;

  IF EXISTS (
    SELECT 1
    FROM control_plane_users users
    WHERE NOT (users.id = 'usr-admin' AND users.account_id = 'acct-admin' AND users.role = 'admin')
      AND (users.id = 'usr-admin' OR users.account_id = 'acct-admin' OR users.role <> 'owner')
  ) THEN
    RAISE EXCEPTION 'legacy customer role is not owner';
  END IF;

  IF EXISTS (
    SELECT 1
    FROM control_plane_users users
    WHERE (SELECT COUNT(*) FROM control_plane_organizations organizations WHERE organizations.billing_account_id = users.account_id) <> 1
  ) THEN
    RAISE EXCEPTION 'legacy customer organization cardinality is not one-to-one';
  END IF;

  IF EXISTS (
    SELECT 1
    FROM control_plane_memberships memberships
    LEFT JOIN control_plane_accounts accounts ON accounts.id = memberships.account_id
    LEFT JOIN control_plane_users users ON users.id = memberships.user_id
    LEFT JOIN control_plane_organizations organizations ON organizations.id = memberships.organization_id
    WHERE accounts.id IS NULL OR users.id IS NULL OR organizations.id IS NULL
      OR users.account_id <> memberships.account_id
      OR organizations.billing_account_id <> memberships.account_id
      OR memberships.role <> 'owner'
  ) OR EXISTS (
    SELECT 1
    FROM control_plane_users users
    WHERE (
      (SELECT COUNT(*) FROM control_plane_memberships memberships WHERE memberships.account_id = users.account_id) <> 1
      OR (SELECT COUNT(*) FROM control_plane_memberships memberships WHERE memberships.user_id = users.id) <> 1
      OR NOT EXISTS (
        SELECT 1
        FROM control_plane_memberships memberships
        JOIN control_plane_organizations organizations ON organizations.id = memberships.organization_id
        WHERE memberships.account_id = users.account_id
          AND memberships.user_id = users.id
          AND organizations.billing_account_id = users.account_id
      )
    )
  ) THEN
    RAISE EXCEPTION 'legacy customer membership is not one-to-one';
  END IF;
END $$;

UPDATE control_plane_accounts accounts
SET owner_user_id = users.id
FROM control_plane_users users
WHERE accounts.owner_user_id = '' AND users.account_id = accounts.id;

DO $$
BEGIN
  IF EXISTS (
    SELECT 1
    FROM control_plane_accounts accounts
    JOIN control_plane_users users ON users.account_id = accounts.id
    WHERE accounts.owner_user_id <> users.id
  ) THEN
    RAISE EXCEPTION 'account owner and user account are not reciprocal';
  END IF;
END $$;

UPDATE control_plane_users
SET email = lower(btrim(email))
WHERE email <> lower(btrim(email));

UPDATE control_plane_users
SET password_hash = ''
WHERE password_hash <> '';

DELETE FROM control_plane_sessions;

CREATE UNIQUE INDEX IF NOT EXISTS control_plane_users_email_normalized_unique
  ON control_plane_users ((lower(btrim(email))));
CREATE UNIQUE INDEX IF NOT EXISTS control_plane_users_account_id_unique
  ON control_plane_users (account_id);
CREATE UNIQUE INDEX IF NOT EXISTS control_plane_users_id_account_id_unique
  ON control_plane_users (id, account_id);
CREATE UNIQUE INDEX IF NOT EXISTS control_plane_accounts_owner_user_id_unique
  ON control_plane_accounts (owner_user_id);
CREATE UNIQUE INDEX IF NOT EXISTS control_plane_accounts_sub2api_user_id_unique
  ON control_plane_accounts (sub2api_user_id);
CREATE UNIQUE INDEX IF NOT EXISTS control_plane_organizations_billing_account_id_unique
  ON control_plane_organizations (billing_account_id);
CREATE UNIQUE INDEX IF NOT EXISTS control_plane_organizations_id_billing_account_id_unique
  ON control_plane_organizations (id, billing_account_id);
CREATE UNIQUE INDEX IF NOT EXISTS control_plane_memberships_account_id_unique
  ON control_plane_memberships (account_id);
CREATE UNIQUE INDEX IF NOT EXISTS control_plane_memberships_user_id_unique
  ON control_plane_memberships (user_id);
CREATE UNIQUE INDEX IF NOT EXISTS control_plane_memberships_organization_id_unique
  ON control_plane_memberships (organization_id);

DO $$
BEGIN
  IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'control_plane_accounts_sub2api_user_id_positive' AND conrelid = 'control_plane_accounts'::regclass) THEN
    ALTER TABLE control_plane_accounts ADD CONSTRAINT control_plane_accounts_sub2api_user_id_positive CHECK (sub2api_user_id > 0);
  END IF;
  IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'control_plane_users_customer_owner_role' AND conrelid = 'control_plane_users'::regclass) THEN
    ALTER TABLE control_plane_users ADD CONSTRAINT control_plane_users_customer_owner_role CHECK (
      (id = 'usr-admin' AND account_id = 'acct-admin' AND role = 'admin')
      OR (id <> 'usr-admin' AND account_id <> 'acct-admin' AND role = 'owner')
    );
  END IF;
  IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'control_plane_users_email_canonical' AND conrelid = 'control_plane_users'::regclass) THEN
    ALTER TABLE control_plane_users ADD CONSTRAINT control_plane_users_email_canonical CHECK (email <> '' AND email = lower(btrim(email)));
  END IF;
  IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'control_plane_users_password_hash_empty' AND conrelid = 'control_plane_users'::regclass) THEN
    ALTER TABLE control_plane_users ADD CONSTRAINT control_plane_users_password_hash_empty CHECK (password_hash = '');
  END IF;
  IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'control_plane_memberships_owner_role' AND conrelid = 'control_plane_memberships'::regclass) THEN
    ALTER TABLE control_plane_memberships ADD CONSTRAINT control_plane_memberships_owner_role CHECK (role = 'owner');
  END IF;
  IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'control_plane_sessions_sub2api_id' AND conrelid = 'control_plane_sessions'::regclass) THEN
    ALTER TABLE control_plane_sessions ADD CONSTRAINT control_plane_sessions_sub2api_id CHECK (id LIKE 'sub2api-sha256:%');
  END IF;
  IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'control_plane_users_account_id_fkey' AND conrelid = 'control_plane_users'::regclass) THEN
    ALTER TABLE control_plane_users ADD CONSTRAINT control_plane_users_account_id_fkey FOREIGN KEY (account_id) REFERENCES control_plane_accounts(id) DEFERRABLE INITIALLY DEFERRED;
  END IF;
  IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'control_plane_accounts_owner_account_fkey' AND conrelid = 'control_plane_accounts'::regclass) THEN
    ALTER TABLE control_plane_accounts ADD CONSTRAINT control_plane_accounts_owner_account_fkey FOREIGN KEY (owner_user_id, id) REFERENCES control_plane_users(id, account_id) DEFERRABLE INITIALLY DEFERRED;
  END IF;
  IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'control_plane_organizations_billing_account_id_fkey' AND conrelid = 'control_plane_organizations'::regclass) THEN
    ALTER TABLE control_plane_organizations ADD CONSTRAINT control_plane_organizations_billing_account_id_fkey FOREIGN KEY (billing_account_id) REFERENCES control_plane_accounts(id) DEFERRABLE INITIALLY DEFERRED;
  END IF;
  IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'control_plane_memberships_account_id_fkey' AND conrelid = 'control_plane_memberships'::regclass) THEN
    ALTER TABLE control_plane_memberships ADD CONSTRAINT control_plane_memberships_account_id_fkey FOREIGN KEY (account_id) REFERENCES control_plane_accounts(id) DEFERRABLE INITIALLY DEFERRED;
  END IF;
  IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'control_plane_memberships_user_account_fkey' AND conrelid = 'control_plane_memberships'::regclass) THEN
    ALTER TABLE control_plane_memberships ADD CONSTRAINT control_plane_memberships_user_account_fkey FOREIGN KEY (user_id, account_id) REFERENCES control_plane_users(id, account_id) DEFERRABLE INITIALLY DEFERRED;
  END IF;
  IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'control_plane_memberships_organization_account_fkey' AND conrelid = 'control_plane_memberships'::regclass) THEN
    ALTER TABLE control_plane_memberships ADD CONSTRAINT control_plane_memberships_organization_account_fkey FOREIGN KEY (organization_id, account_id) REFERENCES control_plane_organizations(id, billing_account_id) DEFERRABLE INITIALLY DEFERRED;
  END IF;
END $$;
