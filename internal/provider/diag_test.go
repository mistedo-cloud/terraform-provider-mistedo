package provider

import (
	"errors"
	"testing"

	"github.com/mistedo/terraform-provider-mistedo/internal/client"
)

func TestDiagFromErr_notFound(t *testing.T) {
	d := diagFromErr("read", client.ErrNotFound)
	if len(d) != 1 || !d.HasError() {
		t.Fatal(d)
	}
	s := d[0].Summary()
	if s != "read" {
		t.Fatal(s)
	}
}

func TestDiagFromErr_apiError(t *testing.T) {
	d := diagFromErr("op", &client.APIError{StatusCode: 400, Message: "bad input"})
	if !d.HasError() {
		t.Fatal("expected error")
	}
}

func TestDiagFromErr_plain(t *testing.T) {
	d := diagFromErr("op", errors.New("plain failure"))
	if !d.HasError() {
		t.Fatal("expected error")
	}
}
