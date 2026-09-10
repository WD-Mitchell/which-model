//go:build !darwin && !linux && !windows

package approvedexec

func Environment() ([]string, error) {
	return nil, refusal("company execution is unavailable on this OS")
}
