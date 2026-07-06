package main

import "testing"

func TestControlPlaneAddrMatchesProductionPortContract(t *testing.T) {
	t.Setenv("CONTROL_PLANE_ADDR", "")
	t.Setenv("PORT", "")
	if got := controlPlaneAddr(); got != ":8787" {
		t.Fatalf("controlPlaneAddr() = %q, want :8787", got)
	}

	t.Setenv("PORT", "8787")
	if got := controlPlaneAddr(); got != ":8787" {
		t.Fatalf("controlPlaneAddr() with PORT = %q, want :8787", got)
	}

	t.Setenv("CONTROL_PLANE_ADDR", ":9000")
	if got := controlPlaneAddr(); got != ":9000" {
		t.Fatalf("controlPlaneAddr() with CONTROL_PLANE_ADDR = %q, want :9000", got)
	}
}
