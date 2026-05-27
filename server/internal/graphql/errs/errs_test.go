package errs

import (
	"errors"
	"fmt"
	"testing"
)

func TestValidationError(t *testing.T) {
	err := Validation("captchaToken", "invalid captcha token")
	if err.Error() != "captchaToken: invalid captcha token" {
		t.Fatalf("unexpected Error(): %q", err.Error())
	}

	var ve *ValidationError
	if !errors.As(err, &ve) {
		t.Fatal("expected errors.As to match *ValidationError")
	}
	if ve.Field != "captchaToken" || ve.Message != "invalid captcha token" {
		t.Fatalf("unexpected fields: field=%q message=%q", ve.Field, ve.Message)
	}
}

func TestValidationErrorWrapped(t *testing.T) {
	wrapped := fmt.Errorf("register failed: %w", Validation("password", "password must be at least 8 characters"))
	ve, ok := AsValidation(wrapped)
	if !ok {
		t.Fatal("expected AsValidation to find a *ValidationError in the chain")
	}
	if ve.Field != "password" {
		t.Fatalf("unexpected field: %q", ve.Field)
	}
}

func TestValidationErrorEmptyField(t *testing.T) {
	err := Validation("", "registration is currently closed")
	if err.Error() != "registration is currently closed" {
		t.Fatalf("unexpected Error() with empty field: %q", err.Error())
	}
}

func TestAsValidationNil(t *testing.T) {
	if _, ok := AsValidation(nil); ok {
		t.Fatal("AsValidation(nil) should return false")
	}
	if _, ok := AsValidation(errors.New("not a validation error")); ok {
		t.Fatal("AsValidation(non-validation) should return false")
	}
}
