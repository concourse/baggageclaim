package baggageclaimcmd

import (
	"fmt"
	"testing"

	"github.com/go-playground/locales/en"
	ut "github.com/go-playground/universal-translator"
	"github.com/go-playground/validator/v10"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

func TestPlanner(t *testing.T) {
	suite.Run(t, &ValidatorTestSuite{
		Assertions: require.New(t),
	})
}

type ValidatorTestSuite struct {
	trans transHelper

	suite.Suite
	*require.Assertions
}

func (v *ValidatorTestSuite) TestValidatorSuite() {
	en := en.New()
	uni := ut.New(en, en)
	trans, _ := uni.GetTranslator("en")
	transHelper := transHelper{trans}

	for _, test := range LogLevelsTests {
		v.Run(test.Title, func() {
			test.TestLogLevelValidator(v, transHelper)
		})
	}

	for _, test := range IPVersionsTests {
		v.Run(test.Title, func() {
			test.TestIPVersionsValidator(v, transHelper)
		})
	}

	for _, test := range BaggageclaimDriverTests {
		v.Run(test.Title, func() {
			test.TestBaggageclaimDriverValidator(v, transHelper)
		})
	}
}

type transHelper struct {
	trans ut.Translator
}

func (t transHelper) RegisterTranslation(validate *validator.Validate, validationName string, errorString string) {
	validate.RegisterTranslation(validationName, t.trans, func(ut ut.Translator) error {
		return ut.Add(validationName, errorString, true) // see universal-translator for details
	}, func(ut ut.Translator, fe validator.FieldError) string {
		t, _ := ut.T(validationName, fe.Field())
		return fmt.Sprintf(`error: %s,
value: %s=%s`, t, fe.Field(), fe.Value())
	})
}

type LogLevelsTest struct {
	Title    string
	LogLevel string
	Valid    bool
}

var LogLevelsTests = []LogLevelsTest{
	{
		Title:    "log level valid choice",
		LogLevel: "debug",
		Valid:    true,
	},
	{
		Title:    "log level invalid choice",
		LogLevel: "invalid-log-level",
		Valid:    false,
	},
}

func (t *LogLevelsTest) TestLogLevelValidator(s *ValidatorTestSuite, trans transHelper) {
	testStruct := struct {
		LogLevel string `validate:"log_level"`
	}{
		LogLevel: t.LogLevel,
	}

	validate := validator.New()
	validate.RegisterValidation("log_level", ValidateLogLevel)
	trans.RegisterTranslation(validate, "log_level", ValidationErrLogLevel)

	err := validate.Struct(testStruct)
	if t.Valid {
		s.Assert().NoError(err)
	} else {
		s.Contains(fmt.Sprintf("%v", err.(validator.ValidationErrors).Translate(trans.trans)), ValidationErrLogLevel)
	}
}

type IPVersionsTest struct {
	Title     string
	IPVersion int
	Valid     bool
}

var IPVersionsTests = []IPVersionsTest{
	{
		Title:     "ip versions valid choice 4",
		IPVersion: 4,
		Valid:     true,
	},
	{
		Title:     "log level valid choice 6",
		IPVersion: 6,
		Valid:     true,
	},
	{
		Title:     "log level invalid choice",
		IPVersion: 3,
		Valid:     false,
	},
}

func (t *IPVersionsTest) TestIPVersionsValidator(s *ValidatorTestSuite, trans transHelper) {
	testStruct := struct {
		IPVersion int `validate:"ip_version"`
	}{
		IPVersion: t.IPVersion,
	}

	validate := validator.New()
	validate.RegisterValidation("ip_version", ValidateIPVersion)
	trans.RegisterTranslation(validate, "ip_version", ValidationErrIPVersion)

	err := validate.Struct(testStruct)
	if t.Valid {
		s.Assert().NoError(err)
	} else {
		s.Contains(fmt.Sprintf("%v", err.(validator.ValidationErrors).Translate(trans.trans)), ValidationErrIPVersion)
	}
}

type BaggageclaimDriverTest struct {
	Title  string
	Driver string
	Valid  bool
}

var BaggageclaimDriverTests = []BaggageclaimDriverTest{
	{
		Title:  "baggageclaim driver valid choice",
		Driver: "overlay",
		Valid:  true,
	},
	{
		Title:  "baggageclaim driver invalid choice",
		Driver: "unknown-driver",
		Valid:  false,
	},
}

func (t *BaggageclaimDriverTest) TestBaggageclaimDriverValidator(s *ValidatorTestSuite, trans transHelper) {
	testStruct := struct {
		Driver string `validate:"baggageclaim_driver"`
	}{
		Driver: t.Driver,
	}

	validate := validator.New()
	validate.RegisterValidation("baggageclaim_driver", ValidateBaggageclaimDriver)
	trans.RegisterTranslation(validate, "baggageclaim_driver", ValidationErrBaggageclaimDriver)

	err := validate.Struct(testStruct)
	if t.Valid {
		s.Assert().NoError(err)
	} else {
		s.Contains(fmt.Sprintf("%v", err.(validator.ValidationErrors).Translate(trans.trans)), ValidationErrBaggageclaimDriver)
	}
}
