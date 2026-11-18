package compose

// IsCompleted reports whether a container is a finished one-shot rather than a stopped
// or failing service: it exited with code 0 and either is a Compose one-off or has
// restart policy "no". An unknown exit code or policy is never completed.
func IsCompleted(state string, exitCode *int, restartPolicy string, oneOff bool) bool {
	if state != "exited" || exitCode == nil || *exitCode != 0 {
		return false
	}
	return oneOff || restartPolicy == "no"
}

// IsFailed reports whether a container exited with a non-zero code.
func IsFailed(state string, exitCode *int) bool {
	return state == "exited" && exitCode != nil && *exitCode != 0
}
