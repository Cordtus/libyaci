package signing

// TestMnemonic is a canonical, publicly known BIP39 mnemonic for tests,
// simulation, and fee estimation. It is eleven "about" words followed by the
// checksum word "abuse" (index 9).
//
// It controls no funds and MUST NOT be used to hold real assets.
const TestMnemonic = "about about about about about about about about about about about abuse"

// NewTestSigner derives a signer from TestMnemonic. Use it for tests,
// simulation, and fee estimation — never for real assets.
func NewTestSigner(algo KeyAlgorithm, opts ...PrivateKeyOption) (*PrivateKeySigner, error) {
	return NewMnemonicSigner(TestMnemonic, DefaultHDPath, algo, opts...)
}
