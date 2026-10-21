package errors

// ErrRepositoryAlreadySet is returned by Config.SetRepository when the config already has a different repository.
// A config has a single repository for its lifetime, so one can never replace another (and lose its features,
// notifiers and usage plugins).
type ErrRepositoryAlreadySet struct{}

func NewErrRepositoryAlreadySet() *ErrRepositoryAlreadySet {
	return &ErrRepositoryAlreadySet{}
}

func (e *ErrRepositoryAlreadySet) Error() string {
	return "config already has a repository"
}
