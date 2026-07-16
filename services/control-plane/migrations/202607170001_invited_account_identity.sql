DO $$
BEGIN
  IF EXISTS (
    SELECT 1
    FROM control_plane_users
    WHERE email IS NULL OR btrim(email) = ''
  ) THEN
    RAISE EXCEPTION 'blank normalized user email';
  END IF;

  IF EXISTS (
    SELECT 1
    FROM control_plane_users
    GROUP BY lower(btrim(email))
    HAVING COUNT(*) > 1
  ) THEN
    RAISE EXCEPTION 'duplicate normalized user emails';
  END IF;
END $$;

UPDATE control_plane_users
SET email = lower(btrim(email))
WHERE email <> lower(btrim(email));

CREATE UNIQUE INDEX IF NOT EXISTS control_plane_users_email_normalized_unique
  ON control_plane_users ((lower(btrim(email))));
