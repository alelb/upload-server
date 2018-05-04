package config

import (
	"github.com/fabbricadigitale/scimd/validation"
	"github.com/fatih/structs"
	d "github.com/mcuadros/go-defaults"
	"github.com/spf13/viper"
	validator "gopkg.in/go-playground/validator.v9"
)

// Configuration is
type Configuration struct {
	Debug bool
	Host  string `default:"" validate:"hostname|ip4_addr"`
	Port  string `default:"8282" validate:"min=1024,max=65535"`
}

var (
	// Values contains the configuration values
	Values *Configuration
	// Errors contains the happened configuration errors
	Errors validator.ValidationErrors
)

func init() {
	getConfig()
}

func getConfig() {
	Values = new(Configuration)

	// Defaults
	d.SetDefaults(Values)
	for key, value := range structs.Map(Values) {
		viper.SetDefault(key, value)
	}

	err := viper.Unmarshal(&Values)
	if err != nil {
		panic(err)
	}

	// Validate the configurations and collect errors
	_, err = Valid()
	if err != nil {
		errs, _ := err.(validator.ValidationErrors)
		Errors = append(Errors, errs...)
	}
}

// Valid checks wheter the configuration is valid or not
func Valid() (bool, error) {
	if err := validation.Validator.Struct(Values); err != nil {
		return false, err
	}
	return true, nil
}
