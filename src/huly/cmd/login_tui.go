package cmd

import (
	"errors"
	"fmt"

	"github.com/charmbracelet/huh"
)

// otpInputs are the values the login form collects.
type otpInputs struct {
	URL       string
	Email     string
	Workspace string
	Save      bool
}

var errLoginCancelled = errors.New("login cancelled")

func required(field string) func(string) error {
	return func(s string) error {
		if s == "" {
			return fmt.Errorf("%s is required", field)
		}
		return nil
	}
}

// collectOTPInputs shows the URL/Email/Workspace form (prefilled) plus a
// "save to config" toggle, and returns the completed inputs.
func collectOTPInputs(prefill otpInputs) (otpInputs, error) {
	in := prefill
	in.Save = true // default the toggle on
	form := huh.NewForm(
		huh.NewGroup(
			huh.NewInput().Title("Huly URL").Value(&in.URL).Validate(required("URL")),
			huh.NewInput().Title("Email").Value(&in.Email).Validate(required("email")),
			huh.NewInput().Title("Workspace").Value(&in.Workspace).Validate(required("workspace")),
			huh.NewConfirm().Title("Save URL/email/workspace to config?").Value(&in.Save),
		).Title("Huly OTP Login"),
	)
	if err := form.Run(); err != nil {
		if errors.Is(err, huh.ErrUserAborted) {
			return otpInputs{}, errLoginCancelled
		}
		return otpInputs{}, err
	}
	return in, nil
}

// promptCodeTUI shows a styled input for the emailed one-time code.
func promptCodeTUI() (string, error) {
	var code string
	form := huh.NewForm(
		huh.NewGroup(
			huh.NewInput().Title("Enter the code sent to your email").Value(&code).Validate(required("code")),
		),
	)
	if err := form.Run(); err != nil {
		if errors.Is(err, huh.ErrUserAborted) {
			return "", errLoginCancelled
		}
		return "", err
	}
	return code, nil
}
