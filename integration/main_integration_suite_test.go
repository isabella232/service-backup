package integration

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
)

const (
	awsAccessKeyIDEnvKey     = "AWS_ACCESS_KEY_ID"
	awsSecretAccessKeyEnvKey = "AWS_SECRET_ACCESS_KEY"

	existingBucketName = "service-backup-integration-test"
	awsTimeout         = "20s"

	awsCLIPath   = "aws"
	endpointURL  = "https://s3.amazonaws.com"
	cronSchedule = "*/5 * * * * *" // every 5 seconds of every minute of every day etc
)

var (
	pathToServiceBackupBinary string
	awsAccessKeyID            string
	awsSecretAccessKey        string
	destPath                  string
)

type config struct {
	AWSAccessKeyID     string `json:"awsAccessKeyID"`
	AWSSecretAccessKey string `json:"awsSecretAccessKey"`
	PathToBackupBinary string `json:"pathToBackupBinary"`
}

func TestServiceBackupBinary(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Service Backup Binary Suite")
}

func beforeSuiteFirstNode() []byte {
	awsAccessKeyID = os.Getenv(awsAccessKeyIDEnvKey)
	awsSecretAccessKey = os.Getenv(awsSecretAccessKeyEnvKey)

	if awsAccessKeyID == "" || awsSecretAccessKey == "" {
		Fail(fmt.Sprintf("Specify valid AWS credentials using the env variables %s and %s", awsAccessKeyIDEnvKey, awsSecretAccessKeyEnvKey))
	}

	var err error
	pathToServiceBackupBinary, err = gexec.Build("github.com/pivotal-cf-experimental/service-backup")
	Expect(err).ToNot(HaveOccurred())

	c := config{
		AWSAccessKeyID:     awsAccessKeyID,
		AWSSecretAccessKey: awsSecretAccessKey,
		PathToBackupBinary: pathToServiceBackupBinary,
	}

	data, err := json.Marshal(c)
	Expect(err).ToNot(HaveOccurred())

	createBucketIfNeeded()

	return data
}

func createBucketIfNeeded() {
	session, err := runS3Command(
		"ls",
		existingBucketName,
	)

	Expect(err).ToNot(HaveOccurred())
	Eventually(session, awsTimeout).Should(gexec.Exit())

	exitCode := session.ExitCode()
	if exitCode != 0 {
		errOut := string(session.Err.Contents())

		if !strings.Contains(errOut, "NoSuchBucket") {
			Fail("Unable to list bucket: " + existingBucketName + " - error: " + errOut)
		}

		session, err := runS3Command(
			"mb",
			"s3://"+existingBucketName,
		)
		Expect(err).ToNot(HaveOccurred())
		Eventually(session, awsTimeout).Should(gexec.Exit(0))
		Eventually(session.Out).Should(gbytes.Say("make_bucket: s3://" + existingBucketName))
	}
}

func runS3Command(args ...string) (*gexec.Session, error) {
	env := []string{}
	env = append(env, fmt.Sprintf("%s=%s", awsAccessKeyIDEnvKey, awsAccessKeyID))
	env = append(env, fmt.Sprintf("%s=%s", awsSecretAccessKeyEnvKey, awsSecretAccessKey))

	verifyBackupCmd := exec.Command(
		awsCLIPath,
		append([]string{
			"s3",
			"--region",
			"us-east-1",
		}, args...)...,
	)
	verifyBackupCmd.Env = env

	return gexec.Start(verifyBackupCmd, GinkgoWriter, GinkgoWriter)
}

func beforeSuiteOtherNodes(b []byte) {
	var c config
	err := json.Unmarshal(b, &c)
	Expect(err).ToNot(HaveOccurred())

	awsAccessKeyID = c.AWSAccessKeyID
	awsSecretAccessKey = c.AWSSecretAccessKey
	pathToServiceBackupBinary = c.PathToBackupBinary
}

var _ = SynchronizedBeforeSuite(beforeSuiteFirstNode, beforeSuiteOtherNodes)

var _ = SynchronizedAfterSuite(func() {
	return
}, func() {
	gexec.CleanupBuildArtifacts()
})

func assetPath(filename string) string {
	path, err := filepath.Abs(filepath.Join("assets", filename))
	Expect(err).ToNot(HaveOccurred())
	return path
}
