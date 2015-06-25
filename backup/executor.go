package backup

import (
	"fmt"
	"os/exec"
	"strings"

	"github.com/pivotal-golang/lager"
)

type Executor interface {
	RunOnce() error
}

type backup struct {
	awsCLIBinaryPath   string
	sourceFolder       string
	destBucket         string
	destPath           string
	awsAccessKeyID     string
	awsSecretAccessKey string
	endpointURL        string
	backupCreatorCmd   string
	cleanupCmd         string
	logger             lager.Logger
}

func NewExecutor(
	awsCLIBinaryPath,
	sourceFolder,
	destBucket,
	destPath,
	awsAccessKeyID,
	awsSecretAccessKey,
	endpointURL,
	backupCreatorCmd,
	cleanupCmd string,
	logger lager.Logger,
) Executor {
	return &backup{
		awsCLIBinaryPath:   awsCLIBinaryPath,
		sourceFolder:       sourceFolder,
		destBucket:         destBucket,
		destPath:           destPath,
		awsAccessKeyID:     awsAccessKeyID,
		awsSecretAccessKey: awsSecretAccessKey,
		endpointURL:        endpointURL,
		backupCreatorCmd:   backupCreatorCmd,
		cleanupCmd:         cleanupCmd,
		logger:             logger,
	}
}

func (b *backup) RunOnce() error {
	err := b.createBucketIfNeeded()
	if err != nil {
		return err
	}

	err = b.performBackup()
	if err != nil {
		return err
	}

	err = b.uploadBackup()
	if err != nil {
		return err
	}

	// Do not return error if cleanup command failed.
	_ = b.performCleanup()
	return nil
}

func (b *backup) createBucketIfNeeded() error {
	b.logger.Info("Checking for bucket", lager.Data{"destBucket": b.destBucket})
	cmd := exec.Command(
		b.awsCLIBinaryPath,
		"s3",
		"ls",
		"--region",
		"us-east-1",
		b.destBucket,
	)

	cmd.Env = []string{}
	cmd.Env = append(cmd.Env, fmt.Sprintf("AWS_ACCESS_KEY_ID=%s", b.awsAccessKeyID))
	cmd.Env = append(cmd.Env, fmt.Sprintf("AWS_SECRET_ACCESS_KEY=%s", b.awsSecretAccessKey))

	out, err := cmd.CombinedOutput()
	if err == nil {
		b.logger.Info("Checking for bucket - bucket exists")
		return nil
	}

	errOut := string(out)

	if !strings.Contains(errOut, "NoSuchBucket") {
		b.logger.Error("Checking for bucket - unable to list bucket", err, lager.Data{"destBucket": b.destBucket})
		return err
	}

	b.logger.Info("Checking for bucket - bucket does not exist - making it now")
	cmd = exec.Command(
		b.awsCLIBinaryPath,
		"s3",
		"mb",
		"--region",
		"us-east-1",
		"s3://"+b.destBucket,
	)

	cmd.Env = []string{}
	cmd.Env = append(cmd.Env, fmt.Sprintf("AWS_ACCESS_KEY_ID=%s", b.awsAccessKeyID))
	cmd.Env = append(cmd.Env, fmt.Sprintf("AWS_SECRET_ACCESS_KEY=%s", b.awsSecretAccessKey))

	out, err = cmd.CombinedOutput()

	if err != nil {
		b.logger.Error("Checking for bucket - Unable to create bucket", err, lager.Data{"destBucket": b.destBucket, "out": string(out)})
		return err
	}
	b.logger.Info("Checking for bucket - bucket created ok")
	return nil
}

func (b *backup) performBackup() error {
	b.logger.Info("Perform backup started")
	args := strings.Split(b.backupCreatorCmd, " ")
	cmd := exec.Command(args[0], args[1:]...)

	out, err := cmd.CombinedOutput()
	b.logger.Debug("Perform backup debug info", lager.Data{"cmd": b.backupCreatorCmd, "out": string(out)})

	if err != nil {
		b.logger.Error("Perform backup completed with error", err)
		return err
	}

	b.logger.Info("Perform backup completed without error")
	return nil
}

func (b *backup) performCleanup() error {
	if b.cleanupCmd == "" {
		b.logger.Info("Cleanup command not provided")
		return nil
	}
	b.logger.Info("Cleanup started")

	args := strings.Split(b.cleanupCmd, " ")
	cmd := exec.Command(args[0], args[1:]...)

	out, err := cmd.CombinedOutput()
	b.logger.Debug("Cleanup debug info", lager.Data{"cmd": b.cleanupCmd, "out": string(out)})

	if err != nil {
		b.logger.Error("Cleanup completed with error", err)
		return err
	}

	b.logger.Info("Cleanup completed without error")
	return nil
}

func (b *backup) uploadBackup() error {
	b.logger.Info("Upload backup started")
	cmd := exec.Command(
		b.awsCLIBinaryPath,
		"s3",
		"sync",
		b.sourceFolder,
		fmt.Sprintf("s3://%s/%s", b.destBucket, b.destPath),
		"--endpoint-url",
		b.endpointURL,
	)

	cmd.Env = []string{}
	cmd.Env = append(cmd.Env, fmt.Sprintf("AWS_ACCESS_KEY_ID=%s", b.awsAccessKeyID))

	b.logger.Debug("Upload backup debug info", lager.Data{"command": cmd})
	cmd.Env = append(cmd.Env, fmt.Sprintf("AWS_SECRET_ACCESS_KEY=%s", b.awsSecretAccessKey))

	out, err := cmd.CombinedOutput()
	b.logger.Debug("Upload backup debug output", lager.Data{"out": string(out)})
	if err != nil {
		b.logger.Error("Upload backup completed with error", err)
		return err
	}

	b.logger.Info("Upload backup completed without error")
	return nil
}
