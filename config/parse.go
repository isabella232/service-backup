package config

import (
	"io/ioutil"

	"code.cloudfoundry.org/lager"
	alerts "github.com/pivotal-cf/service-alerts-client/client"
	"gopkg.in/yaml.v2"
)

func Parse(backupConfigPath string, logger lager.Logger) (BackupConfig, error) {
	configYAML, err := ioutil.ReadFile(backupConfigPath)
	if err != nil {
		logger.Error("error reading config file", err)
		return BackupConfig{}, err
	}
	var backupConfig BackupConfig
	if err := yaml.Unmarshal([]byte(configYAML), &backupConfig); err != nil {
		logger.Error("error unmarshalling config file", err)
		return BackupConfig{}, err
	}

	if !backupConfig.AddDeploymentName {
		backupConfig.DeploymentName = ""
	}

	return backupConfig, nil
}

type Destination struct {
	Type   string                 `yaml:"type"`
	Name   string                 `yaml:"name"`
	Config map[string]interface{} `yaml:"config"`
}

type Alerts struct {
	ProductName string        `yaml:"product_name"`
	Config      alerts.Config `yaml:"config"`
}

type BackupConfig struct {
	Destinations                []Destination `yaml:"destinations"`
	SourceFolder                string        `yaml:"source_folder"`
	SourceExecutable            string        `yaml:"source_executable"`
	CronSchedule                string        `yaml:"cron_schedule"`
	CleanupExecutable           string        `yaml:"cleanup_executable"`
	MissingPropertiesMessage    string        `yaml:"missing_properties_message"`
	ExitIfInProgress            bool          `yaml:"exit_if_in_progress"`
	ServiceIdentifierExecutable string        `yaml:"service_identifier_executable"`
	DeploymentName              string        `yaml:"deployment_name"`
	AddDeploymentName           bool          `yaml:"add_deployment_name_to_backup_path"`
	AwsCliPath                  string        `yaml:"aws_cli_path"`
	AzureCliPath                string        `yaml:"azure_cli_path"`
	Alerts                      *Alerts       `yaml:"alerts,omitempty"`
}

func (b BackupConfig) NoDestinations() bool {
	return len(b.Destinations) == 0
}
