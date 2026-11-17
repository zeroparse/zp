package codegentest

import "testing"

func TestWithWitnessTag(t *testing.T) {
	for in, want := range map[string]string{
		"":                     "-tags=zp_feature_witness",
		"-trimpath":            "-trimpath -tags=zp_feature_witness",
		"-tags=zp_x -trimpath": "-tags=zp_x,zp_feature_witness -trimpath",
		"-trimpath --tags=a,b": "-trimpath --tags=a,b,zp_feature_witness",
		"-tags= -mod=mod":      "-tags=zp_feature_witness -mod=mod",
	} {
		if got := WithWitnessTag(in); got != want {
			t.Errorf("WithWitnessTag(%q) = %q, want %q", in, got, want)
		}
	}
}
