package encryptor

type Encryptor interface {
	Encrypt(data []byte) ([]byte, error)
	Decrypt(data []byte) ([]byte, error)
}

type Key struct {
	Name         string
	ID           string
	Value        []byte //used in NaCl
	PublicValue  []byte // used in PGP
	PrivateValue []byte // usef in PGP
}

func (key Key) Public() Key {
	pub := Key{
		Name:        key.Name,
		ID:          key.ID,
		PublicValue: key.PublicValue,
	}
	return pub
}

type KeyDescription struct {
	Name string
	ID   string
}
