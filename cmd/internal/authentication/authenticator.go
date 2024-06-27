package authentication

type Authenticator interface {
	RetrievePassword() (string, error)
}
