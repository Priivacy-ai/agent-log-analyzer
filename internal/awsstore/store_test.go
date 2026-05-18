package awsstore

import "testing"

func TestParseS3Path(t *testing.T) {
	bucket, key, err := parseS3Path("s3://uploads/uploads/job-1.log")
	if err != nil {
		t.Fatalf("parseS3Path failed: %v", err)
	}
	if bucket != "uploads" || key != "uploads/job-1.log" {
		t.Fatalf("unexpected parse result bucket=%q key=%q", bucket, key)
	}
}

func TestParseS3PathRejectsInvalid(t *testing.T) {
	for _, path := range []string{"", "/tmp/file", "s3://bucket", "s3:///key"} {
		if _, _, err := parseS3Path(path); err == nil {
			t.Fatalf("expected error for %q", path)
		}
	}
}
