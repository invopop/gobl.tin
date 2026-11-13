package main

import (
	"os"

	tin "github.com/invopop/gobl.tin"
	"github.com/invopop/gobl.tin/hmrc"
	"github.com/spf13/pflag"
)

// envHMRCSecret names the environment variable with the client secret of
// the HMRC application, so that the secret never appears in the process
// list.
const envHMRCSecret = "HMRC_CLIENT_SECRET"

// hmrcRegister wires HMRC. It runs only when a client ID is given, because
// HMRC requires the credentials of an application.
type hmrcRegister struct {
	clientID string
	sandbox  bool
}

func (r *hmrcRegister) flags(fs *pflag.FlagSet) {
	fs.StringVar(&r.clientID, "hmrc-client-id", "", "Client ID of an HMRC application, which then verifies UK VAT numbers; the secret is read from "+envHMRCSecret)
	fs.BoolVar(&r.sandbox, "hmrc-sandbox", false, "The HMRC application is a sandbox application")
}

func (r *hmrcRegister) verifier() (tin.Verifier, error) {
	if r.clientID == "" {
		return nil, nil
	}
	var opts []hmrc.Option
	if r.sandbox {
		opts = append(opts, hmrc.WithSandbox())
	}
	return hmrc.New(r.clientID, os.Getenv(envHMRCSecret), opts...), nil
}
