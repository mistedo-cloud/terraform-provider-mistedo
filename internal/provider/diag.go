package provider

import (
	"errors"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/mistedo/terraform-provider-mistedo/internal/client"
)

func diagFromErr(summary string, err error) diag.Diagnostics {
	var d diag.Diagnostics
	if err == nil {
		return d
	}
	if errors.Is(err, client.ErrNotFound) {
		d.AddError(summary, "The resource no longer exists on the platform. It may have been removed outside Terraform.")
		return d
	}
	var api *client.APIError
	if errors.As(err, &api) {
		detail := api.UserFacing()
		if api.StatusCode > 0 {
			detail = fmt.Sprintf("HTTP %d: %s", api.StatusCode, detail)
		}
		d.AddError(summary, detail)
		return d
	}
	d.AddError(summary, err.Error())
	return d
}

func diagAPI(summary string, err error) diag.Diagnostics {
	return diagFromErr(summary, err)
}
