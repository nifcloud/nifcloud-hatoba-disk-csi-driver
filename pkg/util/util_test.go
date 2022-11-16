package util

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRoundUpBytes(t *testing.T) {
	tests := []struct {
		volumeSizeBytes int64
		want            int64
	}{
		{
			volumeSizeBytes: 0,
			want:            0,
		},
		{
			volumeSizeBytes: 1,
			want:            1 * GiB,
		},
		{
			volumeSizeBytes: GiB,
			want:            1 * GiB,
		},
		{
			volumeSizeBytes: GiB + 1,
			want:            2 * GiB,
		},
	}
	for _, tt := range tests {
		got := RoundUpBytes(tt.volumeSizeBytes)
		assert.Equal(t, tt.want, got)
	}
}

func TestRoundUpGiB(t *testing.T) {
	tests := []struct {
		volumeSizeBytes int64
		want            int64
	}{
		{
			volumeSizeBytes: 0,
			want:            0,
		},
		{
			volumeSizeBytes: 1,
			want:            1,
		},
		{
			volumeSizeBytes: GiB,
			want:            1,
		},
		{
			volumeSizeBytes: GiB + 1,
			want:            2,
		},
	}
	for _, tt := range tests {
		got := RoundUpGiB(tt.volumeSizeBytes)
		assert.Equal(t, tt.want, got)
	}
}

func TestBytesToGiB(t *testing.T) {
	tests := []struct {
		volumeSizeBytes int64
		want            int64
	}{
		{
			volumeSizeBytes: 1,
			want:            0,
		},
		{
			volumeSizeBytes: GiB - 1,
			want:            0,
		},
		{
			volumeSizeBytes: GiB,
			want:            1,
		},
		{
			volumeSizeBytes: GiB + 1,
			want:            1,
		},
		{
			volumeSizeBytes: 10 * GiB,
			want:            10,
		},
	}
	for _, tt := range tests {
		got := BytesToGiB(tt.volumeSizeBytes)
		assert.Equal(t, tt.want, got)
	}
}

func TestGiBToBytes(t *testing.T) {
	var volumeSizeGiB int64 = 10
	var want int64 = 10 * GiB
	got := GiBToBytes(volumeSizeGiB)
	assert.Equal(t, want, got)
}
