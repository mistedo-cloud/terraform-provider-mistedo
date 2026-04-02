package provider

import (
	"context"
	"fmt"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
)

// dnsRecordTypes lists supported RR types (Mistedo DNS API).
var dnsRecordTypes = map[string]struct{}{
	"A": {}, "AAAA": {}, "CNAME": {}, "TXT": {}, "SRV": {}, "MX": {}, "NS": {},
}

type dnsRecordTypeValidator struct{}

var _ validator.String = dnsRecordTypeValidator{}

func (v dnsRecordTypeValidator) Description(_ context.Context) string {
	return "must be one of A, AAAA, CNAME, TXT, SRV, MX, or NS"
}

func (v dnsRecordTypeValidator) MarkdownDescription(_ context.Context) string {
	return "Must be one of `A`, `AAAA`, `CNAME`, `TXT`, `SRV`, `MX`, or `NS` (case-insensitive)."
}

func (v dnsRecordTypeValidator) ValidateString(_ context.Context, req validator.StringRequest, resp *validator.StringResponse) {
	if req.ConfigValue.IsNull() || req.ConfigValue.IsUnknown() {
		return
	}
	t := strings.ToUpper(strings.TrimSpace(req.ConfigValue.ValueString()))
	if _, ok := dnsRecordTypes[t]; !ok {
		resp.Diagnostics.AddAttributeError(
			req.Path,
			"Invalid DNS record type",
			fmt.Sprintf("Got %q; expected one of: A, AAAA, CNAME, TXT, SRV, MX, NS.", req.ConfigValue.ValueString()),
		)
	}
}

// zoneFQDNNoTrailingDotValidator rejects FQDNs ending with "." (the API expects names without a final dot).
type zoneFQDNNoTrailingDotValidator struct{}

var _ validator.String = zoneFQDNNoTrailingDotValidator{}

func (v zoneFQDNNoTrailingDotValidator) Description(_ context.Context) string {
	return "must not end with a trailing dot"
}

func (v zoneFQDNNoTrailingDotValidator) MarkdownDescription(_ context.Context) string {
	return "Must not end with a trailing dot. The Mistedo DNS API uses zone names without a final dot (e.g. `example.com`, not `example.com.`)."
}

func (v zoneFQDNNoTrailingDotValidator) ValidateString(_ context.Context, req validator.StringRequest, resp *validator.StringResponse) {
	if req.ConfigValue.IsNull() || req.ConfigValue.IsUnknown() {
		return
	}
	s := strings.TrimSpace(req.ConfigValue.ValueString())
	if strings.HasSuffix(s, ".") {
		resp.Diagnostics.AddAttributeError(
			req.Path,
			"Invalid zone FQDN",
			"Remove the trailing dot. The Mistedo DNS API expects zone names without a final dot (e.g. `example.com`, not `example.com.`).",
		)
	}
}
