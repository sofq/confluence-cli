package policy_test

import (
	"errors"
	"testing"

	"github.com/sofq/confluence-cli/internal/policy"
)

func TestNewFromConfig_NilNil_ReturnsNilPolicy(t *testing.T) {
	p, err := policy.NewFromConfig(nil, nil)
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if p != nil {
		t.Fatalf("expected nil policy, got %v", p)
	}
}

func TestNewFromConfig_AllowOnly_ReturnsPolicyAllowMode(t *testing.T) {
	p, err := policy.NewFromConfig([]string{"pages:*"}, nil)
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if p == nil {
		t.Fatal("expected non-nil policy")
	}
}

func TestNewFromConfig_DenyOnly_ReturnsPolicyDenyMode(t *testing.T) {
	p, err := policy.NewFromConfig(nil, []string{"pages:create"})
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if p == nil {
		t.Fatal("expected non-nil policy")
	}
}

func TestNewFromConfig_BothAllowAndDeny_ReturnsError(t *testing.T) {
	p, err := policy.NewFromConfig([]string{"a"}, []string{"b"})
	if err == nil {
		t.Fatal("expected error for conflicting allow and deny, got nil")
	}
	if p != nil {
		t.Fatalf("expected nil policy on error, got %v", p)
	}
}

func TestNewFromConfig_InvalidGlob_ReturnsError(t *testing.T) {
	p, err := policy.NewFromConfig([]string{"[invalid"}, nil)
	if err == nil {
		t.Fatal("expected error for invalid glob, got nil")
	}
	if p != nil {
		t.Fatalf("expected nil policy on error, got %v", p)
	}
}

func TestNilPolicy_Check_ReturnsNil(t *testing.T) {
	var p *policy.Policy
	if err := p.Check("anything"); err != nil {
		t.Fatalf("expected nil from nil policy, got %v", err)
	}
}

func TestAllowPolicy_Check_MatchingOp_ReturnsNil(t *testing.T) {
	p, err := policy.NewFromConfig([]string{"pages:*"}, nil)
	if err != nil || p == nil {
		t.Fatalf("setup failed: err=%v, p=%v", err, p)
	}
	if err := p.Check("pages:get"); err != nil {
		t.Fatalf("expected allowed op to pass, got %v", err)
	}
}

func TestAllowPolicy_Check_NonMatchingOp_ReturnsDeniedError(t *testing.T) {
	p, err := policy.NewFromConfig([]string{"pages:*"}, nil)
	if err != nil || p == nil {
		t.Fatalf("setup failed: err=%v, p=%v", err, p)
	}
	err = p.Check("spaces:list")
	if err == nil {
		t.Fatal("expected DeniedError, got nil")
	}
	var de *policy.DeniedError
	if !errors.As(err, &de) {
		t.Fatalf("expected *DeniedError, got %T", err)
	}
}

func TestDenyPolicy_Check_MatchingOp_ReturnsDeniedError(t *testing.T) {
	p, err := policy.NewFromConfig(nil, []string{"pages:create"})
	if err != nil || p == nil {
		t.Fatalf("setup failed: err=%v, p=%v", err, p)
	}
	err = p.Check("pages:create")
	if err == nil {
		t.Fatal("expected DeniedError, got nil")
	}
	var de *policy.DeniedError
	if !errors.As(err, &de) {
		t.Fatalf("expected *DeniedError, got %T", err)
	}
}

func TestDenyPolicy_Check_NonMatchingOp_ReturnsNil(t *testing.T) {
	p, err := policy.NewFromConfig(nil, []string{"pages:create"})
	if err != nil || p == nil {
		t.Fatalf("setup failed: err=%v, p=%v", err, p)
	}
	if err := p.Check("pages:get"); err != nil {
		t.Fatalf("expected non-denied op to pass, got %v", err)
	}
}

func TestDeniedError_Error_ContainsOperation(t *testing.T) {
	p, err := policy.NewFromConfig([]string{"pages:*"}, nil)
	if err != nil || p == nil {
		t.Fatalf("setup failed: err=%v, p=%v", err, p)
	}
	checkErr := p.Check("spaces:list")
	if checkErr == nil {
		t.Fatal("expected error, got nil")
	}
	if msg := checkErr.Error(); len(msg) == 0 {
		t.Fatal("error message is empty")
	}
	var de *policy.DeniedError
	if errors.As(checkErr, &de) {
		if de.Operation != "spaces:list" {
			t.Fatalf("expected operation 'spaces:list' in DeniedError, got %q", de.Operation)
		}
	}
}
