package gcp_test

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"time"

	"cloud.google.com/go/storage"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/pborman/uuid"
	"github.com/pivotal-cf-experimental/service-backup/gcp"
	"github.com/pivotal-golang/lager"
	"google.golang.org/api/option"
)

var _ = Describe("backups to Google Cloud Storage", func() {
	var (
		bucketName     string
		bucket         *storage.BucketHandle
		dirToBackup    string
		ctx            context.Context
		gcpProjectName string

		backuper *gcp.StorageClient
	)

	itBacksUpFiles := func() {
		It("backs up files", func() {
			Expect(readObject(ctx, bucket, "a.txt")).To(Equal("content for a.txt"))
			Expect(readObject(ctx, bucket, "d1/b.txt")).To(Equal("content for b.txt"))
			Expect(readObject(ctx, bucket, "d1/d2/c.txt")).To(Equal("content for c.txt"))
		})
	}

	BeforeEach(func() {
		gcpServiceAccountFilePath := envMustHave("SERVICE_BACKUP_TESTS_GCP_SERVICE_ACCOUNT_FILE")
		gcpProjectName = envMustHave("SERVICE_BACKUP_TESTS_GCP_PROJECT_NAME")

		var err error
		dirToBackup, err = ioutil.TempDir("", "gcp-backup-tests")
		Expect(err).NotTo(HaveOccurred())
		Expect(createFile("content for a.txt", dirToBackup, "a.txt"))
		Expect(createFile("content for b.txt", dirToBackup, "d1", "b.txt"))
		Expect(createFile("content for c.txt", dirToBackup, "d1", "d2", "c.txt"))

		ctx = context.Background()
		gcpClient, err := storage.NewClient(ctx, option.WithServiceAccountFile(gcpServiceAccountFilePath))
		Expect(err).NotTo(HaveOccurred())
		bucketName = fmt.Sprintf("service-backup-test-%s", uuid.New())
		bucket = gcpClient.Bucket(bucketName)

		backuper = gcp.New(gcpServiceAccountFilePath, gcpProjectName, bucketName)
	})

	JustBeforeEach(func() {
		logger := lager.NewLogger("[GCP tests] ")
		logger.RegisterSink(lager.NewWriterSink(GinkgoWriter, lager.DEBUG))
		Expect(backuper.Upload(dirToBackup, logger)).To(Succeed())
	})

	AfterEach(func() {
		Expect(os.RemoveAll(dirToBackup)).To(Succeed())
		deleteBucket(ctx, bucket)
	})

	itBacksUpFiles()

	Context("when the bucket already exists", func() {
		BeforeEach(func() {
			Expect(bucket.Create(ctx, gcpProjectName, nil)).To(Succeed())
		})

		itBacksUpFiles()
	})
})

func envMustHave(key string) string {
	val := os.Getenv(key)
	Expect(val).NotTo(BeEmpty(), fmt.Sprintf("must set %s", key))
	return val
}

func createFile(content string, nameParts ...string) error {
	fullPath, err := ensureDirExists(nameParts)
	if err != nil {
		return err
	}
	return ioutil.WriteFile(fullPath, []byte(content), 0644)
}

func ensureDirExists(nameParts []string) (string, error) {
	fullPath := filepath.Join(nameParts...)
	if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
		return "", err
	}
	return fullPath, nil
}

func deleteBucket(ctx context.Context, bucket *storage.BucketHandle) {
	objectsInBucket := bucket.Objects(ctx, nil)
	for {
		obj, err := objectsInBucket.Next()
		if err == storage.Done {
			break
		}
		Expect(err).NotTo(HaveOccurred())
		Expect(bucket.Object(obj.Name).Delete(ctx)).To(Succeed())
	}
	Expect(bucket.Delete(ctx)).To(Succeed())
}

func readObject(ctx context.Context, bucket *storage.BucketHandle, relativePath string) string {
	bucketObj := bucket.Object(expectedNameInBucket(relativePath))
	objReader, err := bucketObj.NewReader(ctx)
	Expect(err).NotTo(HaveOccurred())
	defer objReader.Close()

	remoteContents := new(bytes.Buffer)
	_, err = io.Copy(remoteContents, objReader)
	Expect(err).NotTo(HaveOccurred())
	return remoteContents.String()
}

func expectedNameInBucket(relativePath string) string {
	today := time.Now()
	return fmt.Sprintf("%d/%02d/%02d/%s", today.Year(), today.Month(), today.Day(), relativePath)
}