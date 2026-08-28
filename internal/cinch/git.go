package cinch

import "cinch/internal/gitutil"

func IsGitRepo(repoRoot string) bool {
	return gitutil.IsRepo(repoRoot)
}
