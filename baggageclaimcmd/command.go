package baggageclaimcmd

import (
	"errors"
	"fmt"
	"os"

	"github.com/clarafu/envstruct"
	"github.com/concourse/flag"
	"github.com/go-playground/locales/en"
	ut "github.com/go-playground/universal-translator"
	val "github.com/go-playground/validator/v10"
	"github.com/hashicorp/go-multierror"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v2"
)

var baggageclaimCmd BaggageclaimCommandFlags

// Baggageclaim command is only used for when the user wants to run
// baggageclaim independently from the worker command. This is not included in
// the concourse commands
var BaggageclaimCommand = &cobra.Command{
	Use:   "baggageclaim",
	Short: "TODO",
	Long:  `TODO`,
	RunE:  InitializeBaggageclaim,
}

func init() {
	BaggageclaimCommand.Flags().Var(&baggageclaimCmd.ConfigFile, "config", "config file (default is $HOME/.cobra.yaml)")

	baggageclaimCmd.BaggageclaimConfig = CmdDefaults

	InitializeBaggageclaimFlags(BaggageclaimCommand, &baggageclaimCmd.BaggageclaimConfig, "")
}

type BaggageclaimCommandFlags struct {
	ConfigFile flag.File `env:"BAGGAGECLAIM_CONFIG_FILE"`

	BaggageclaimConfig
}

func InitializeBaggageclaim(cmd *cobra.Command, args []string) error {
	// Fetch out the values set from the config file and overwrite the flag
	// values
	if baggageclaimCmd.ConfigFile != "" {
		file, err := os.Open(string(baggageclaimCmd.ConfigFile))
		if err != nil {
			return fmt.Errorf("open file: %s", err)
		}

		decoder := yaml.NewDecoder(file)
		err = decoder.Decode(&baggageclaimCmd)
		if err != nil {
			return fmt.Errorf("decode config: %s", err)
		}
	}

	// Validate the values passed in by the user
	en := en.New()
	uni := ut.New(en, en)
	trans, _ := uni.GetTranslator("en")

	validator := NewValidator(trans)

	err = validator.Struct(baggageclaimCmd)
	if err != nil {
		validationErrors := err.(val.ValidationErrors)

		var errs *multierror.Error
		for _, validationErr := range validationErrors {
			errs = multierror.Append(
				errs,
				errors.New(validationErr.Translate(trans)),
			)
		}

		return errs.ErrorOrNil()
	}

	err = baggageclaimCmd.Execute(args)
	if err != nil {
		return fmt.Errorf("failed to execute baggageclaim: %s", err)
	}

	return nil
}
