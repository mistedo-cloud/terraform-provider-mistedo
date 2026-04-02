package provider

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func TestZoneFQDNNoTrailingDotValidator(t *testing.T) {
	v := zoneFQDNNoTrailingDotValidator{}
	for _, tc := range []struct {
		val  string
		want int // number of errors
	}{
		{"example.com", 0},
		{"tf-e2e.ha001.dev.mistedo.by", 0},
		{"example.com.", 1},
		{"  z.test.  ", 1},
	} {
		var resp validator.StringResponse
		v.ValidateString(context.Background(), validator.StringRequest{
			Path: path.Root("name"),
			ConfigValue: types.StringValue(tc.val),
		}, &resp)
		if len(resp.Diagnostics.Errors()) != tc.want {
			t.Fatalf("val=%q: got %d errors, want %d: %v", tc.val, len(resp.Diagnostics.Errors()), tc.want, resp.Diagnostics)
		}
	}
}
