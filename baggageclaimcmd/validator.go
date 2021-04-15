package baggageclaimcmd

import (
	"fmt"

	"github.com/concourse/baggageclaim"
	"github.com/concourse/flag"
	ut "github.com/go-playground/universal-translator"
	"github.com/go-playground/validator/v10"
	en_translations "github.com/go-playground/validator/v10/translations/en"
)

var (
	ValidationErrLogLevel           = fmt.Sprintf("Not a valid log level. Valid options include %v.", flag.ValidLogLevels)
	ValidationErrIPVersion          = fmt.Sprintf("Not a valid IP version. Valid options include 4 for IPv4 or 6 for IPv6.")
	ValidationErrBaggageclaimDriver = fmt.Sprintf("Not a valid baggageclaim driver. Valid options include %v.", baggageclaim.ValidDrivers)
)

func NewValidator(trans ut.Translator) *validator.Validate {
	validate := validator.New()
	en_translations.RegisterDefaultTranslations(validate, trans)

	validate.RegisterValidation("log_level", ValidateLogLevel)
	validate.RegisterValidation("ip_version", ValidateIPVersion)
	validate.RegisterValidation("baggageclaim_driver", ValidateBaggageclaimDriver)

	ve := NewValidatorErrors(validate, trans)
	ve.SetupErrorMessages()

	return validate
}

type validatorErrors struct {
	validate *validator.Validate
	trans    ut.Translator
}

func NewValidatorErrors(validate *validator.Validate, trans ut.Translator) *validatorErrors {
	return &validatorErrors{
		validate: validate,
		trans:    trans,
	}
}

func (v *validatorErrors) SetupErrorMessages() {
	v.RegisterTranslation("log_level", ValidationErrLogLevel)
	v.RegisterTranslation("ip_version", ValidationErrIPVersion)
	v.RegisterTranslation("baggageclaim_driver", ValidationErrBaggageclaimDriver)
}

func (v *validatorErrors) RegisterTranslation(validationName string, errorString string) {
	v.validate.RegisterTranslation(validationName, v.trans, func(ut ut.Translator) error {
		return ut.Add(validationName, errorString, true) // see universal-translator for details
	}, func(ut ut.Translator, fe validator.FieldError) string {
		t, _ := ut.T(validationName, fe.Field())
		return fmt.Sprintf(`error: %s,
value: %s=%s`, t, fe.Field(), fe.Value())
	})
}

func ValidateLogLevel(field validator.FieldLevel) bool {
	value := field.Field().String()
	for _, validChoice := range flag.ValidLogLevels {
		if value == string(validChoice) {
			return true
		}
	}

	return false
}

func ValidateIPVersion(field validator.FieldLevel) bool {
	value := field.Field().Interface()

	if value.(int) == 4 || value.(int) == 6 {
		return true
	} else {
		return false
	}
}

func ValidateBaggageclaimDriver(field validator.FieldLevel) bool {
	value := field.Field().String()
	for _, validChoice := range baggageclaim.ValidDrivers {
		if value == string(validChoice) {
			return true
		}
	}

	return false
}
