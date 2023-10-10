package externalcloudaudit

import "testing"

func TestValidateBuckets(t *testing.T) {
	tt := []struct {
		desc        string
		buckets     []string
		errExpected bool
	}{
		{
			desc:        "multiple buckets multiple prefixes",
			buckets:     []string{"s3://bucket1/prefix1", "s3://bucket2/prefix2", "s3://bucket3/prefix3", "s3://bucket4/prefix4"},
			errExpected: false,
		},
		{
			desc:        "one bucket multiple prefixes",
			buckets:     []string{"s3://bucket1/prefix1", "s3://bucket1/prefix2", "s3://bucket1/prefix3", "s3://bucket1/prefix4"},
			errExpected: false,
		},
		{
			desc:        "one bucket multiple prefixes with two overlapped prefixes",
			buckets:     []string{"s3://bucket1/prefix1", "s3://bucket1/prefix1", "s3://bucket1/prefix3", "s3://bucket1/prefix4"},
			errExpected: true,
		},
		{
			desc:        "one bucket with a root prefix and other non root prefixes",
			buckets:     []string{"s3://bucket1", "s3://bucket1/prefix1", "s3://bucket1/prefix3", "s3://bucket1/prefix4"},
			errExpected: true,
		},
		{
			desc:        "multiple buckets with all root prefixes",
			buckets:     []string{"s3://bucket1", "s3://bucket2/", "s3://bucket3", "s3://bucket4"},
			errExpected: false,
		},
		{
			desc:        "one bucket one prefix is subdirectory of other",
			buckets:     []string{"s3://bucket1/prefix1/", "s3://bucket1/prefix1/prefix2", "s3://bucket2/prefix3", "s3://bucket3/prefix4"},
			errExpected: true,
		},
	}

	for _, tc := range tt {
		t.Run(tc.desc, func(t *testing.T) {
			err := ValidateBuckets(tc.buckets...)
			if err != nil {
				if !tc.errExpected {
					t.Error(err)
				}
			} else {
				if tc.errExpected {
					t.Error("Expected error wasn't called")
				}
			}
		})
	}
}
