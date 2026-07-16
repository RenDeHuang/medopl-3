package server

import (
	"context"
	"reflect"
	"strconv"
	"testing"
)

func TestBootstrapDisabledDeletedAndRevokedStatesSurviveRestart(t *testing.T) {
	for index, mode := range []string{"disabled", "deleted", "revoked"} {
		t.Run(mode, func(t *testing.T) {
			accountID := "acct-bootstrap-" + mode
			userID := "usr-bootstrap-" + mode
			email := "bootstrap-" + mode + "@example.com"
			t.Setenv("OPL_CONSOLE_USERS_JSON", `[{"id":"`+userID+`","email":"`+email+`","password":"CorrectHorseBatteryStaple!","role":"owner","accountId":"`+accountID+`","sub2apiUserId":`+strconv.Itoa(81+index)+`}]`)
			store := newMemoryTableStore()
			first, err := newControlPlaneAppWithStore(store)
			if err != nil {
				t.Fatal(err)
			}
			var expected map[string]any
			switch mode {
			case "disabled":
				expected, err = first.disableUser(map[string]any{"userId": userID, "disabledBy": "usr-admin", "reason": "bootstrap_test"})
			case "deleted":
				expected, err = first.softDeleteUser(map[string]any{"userId": userID, "deletedBy": "usr-admin", "reason": "bootstrap_test"})
			default:
				expected, err = first.findUserByID(context.Background(), userID)
			}
			if err != nil {
				t.Fatal(err)
			}
			memberships, _ := store.ListMemberships(context.Background())
			for _, membership := range memberships {
				if membership["userId"] == userID {
					if _, err := first.revokeMembership(context.Background(), stringValue(membership["id"])); err != nil {
						t.Fatal(err)
					}
				}
			}

			second, err := newControlPlaneAppWithStore(store)
			if err != nil {
				t.Fatal(err)
			}
			actual, err := second.findUserByID(context.Background(), userID)
			if err != nil || actual == nil || actual["status"] != expected["status"] {
				t.Fatalf("restarted user=%#v expected=%#v err=%v", actual, expected, err)
			}
			for _, key := range []string{"disabledAt", "disabledBy", "disabledReason", "deletedAt", "deletedBy", "deleteReason"} {
				if actual[key] != expected[key] {
					t.Fatalf("restarted %s=%#v, want %#v", key, actual[key], expected[key])
				}
			}
			memberships, _ = store.ListMemberships(context.Background())
			matching, active := 0, 0
			for _, membership := range memberships {
				if membership["userId"] == userID {
					matching++
					if membership["status"] == "active" {
						active++
					}
				}
			}
			if matching != 1 || active != 0 {
				t.Fatalf("restarted memberships=%#v", memberships)
			}
			if mode != "revoked" {
				if _, _, err := second.login(map[string]any{"email": email, "password": "CorrectHorseBatteryStaple!"}); err == nil {
					t.Fatal("inactive bootstrap user logged in after restart")
				}
			}
		})
	}
}

func TestBootstrapIdentityConflictFailsClosed(t *testing.T) {
	store := newMemoryTableStore()
	mustStore(t, store.SaveAccount(context.Background(), map[string]any{"id": "acct-bootstrap-one", "status": "active", "sub2apiUserId": int64(111)}))
	mustStore(t, store.SaveAccount(context.Background(), map[string]any{"id": "acct-bootstrap-two", "status": "active", "sub2apiUserId": int64(112)}))
	mustStore(t, store.SaveUser(context.Background(), map[string]any{"id": "usr-bootstrap-one", "email": "one@example.com", "accountId": "acct-bootstrap-one", "role": "owner", "status": "active"}))
	mustStore(t, store.SaveUser(context.Background(), map[string]any{"id": "usr-bootstrap-two", "email": "two@example.com", "accountId": "acct-bootstrap-two", "role": "owner", "status": "active"}))
	beforeAccounts, beforeUsers := cloneStateTable(store.accounts), cloneStateTable(store.users)
	t.Setenv("OPL_CONSOLE_USERS_JSON", `[{"id":"usr-bootstrap-one","email":"two@example.com","password":"CorrectHorseBatteryStaple!","role":"owner","accountId":"acct-bootstrap-conflict","sub2apiUserId":114}]`)

	if _, err := newControlPlaneAppWithStore(store); err == nil || err.Error() != "bootstrap_user_identity_conflict" {
		t.Fatalf("bootstrap identity conflict error = %v", err)
	}
	if !reflect.DeepEqual(store.accounts, beforeAccounts) || !reflect.DeepEqual(store.users, beforeUsers) {
		t.Fatalf("bootstrap identity conflict mutated facts: accounts=%#v users=%#v", store.accounts, store.users)
	}
}
