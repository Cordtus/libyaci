package signing

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"strings"
	"testing"

	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/btcec/v2/ecdsa"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protodesc"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"
	"google.golang.org/protobuf/types/descriptorpb"
	"google.golang.org/protobuf/types/dynamicpb"
)

func TestBuildAndSign(t *testing.T) {
	resolver := testResolver{testFiles(t)}
	signer, err := NewPrivateKeySigner(privKeyOneHex, Secp256k1)
	if err != nil {
		t.Fatalf("NewPrivateKeySigner failed: %v", err)
	}

	msg := Msg(`{"@type":"/test.v1.MsgSend","fromAddress":"cosmos1aaa","toAddress":"cosmos1bbb","amount":[{"denom":"uatom","amount":"5"}]}`)
	tx, err := BuildAndSign(resolver, signer, "test-chain-1", []Msg{msg}, TxOptions{
		Memo:          "hello",
		Fee:           []Coin{{Denom: "uatom", Amount: "1000"}},
		GasLimit:      200000,
		AccountNumber: 7,
		Sequence:      3,
		AddressPrefix: "cosmos",
	})
	if err != nil {
		t.Fatalf("BuildAndSign failed: %v", err)
	}

	if len(tx.Signature) != 64 {
		t.Fatalf("signature length = %d, want 64", len(tx.Signature))
	}
	if len(tx.TxBytes) == 0 || len(tx.SignDocBytes) == 0 {
		t.Fatal("tx bytes and sign doc must be non-empty")
	}
	wantHash := strings.ToUpper(hex.EncodeToString(sha256Sum(tx.TxBytes)))
	if tx.TxHash != wantHash {
		t.Fatalf("TxHash = %s, want %s", tx.TxHash, wantHash)
	}
	address, _ := signer.Address("cosmos")
	if tx.SignerAddress != address {
		t.Fatalf("SignerAddress = %q, want %q", tx.SignerAddress, address)
	}

	// The signature must verify over the exact sign-doc bytes.
	digest := sha256.Sum256(tx.SignDocBytes)
	var r, s btcec.ModNScalar
	if r.SetByteSlice(tx.Signature[:32]) || s.SetByteSlice(tx.Signature[32:]) {
		t.Fatal("signature scalar overflow")
	}
	parsed := ecdsa.NewSignature(&r, &s)
	pub, err := btcec.ParsePubKey(signer.PublicKey())
	if err != nil {
		t.Fatalf("ParsePubKey failed: %v", err)
	}
	if !parsed.Verify(digest[:], pub) {
		t.Fatal("signature did not verify over the sign doc")
	}

	// TxBody carries the message and memo.
	body := decodeDynamic(t, resolver, txBodyType, tx.BodyBytes)
	bodyFields := body.Descriptor().Fields()
	if got := body.Get(bodyFields.ByName("memo")).String(); got != "hello" {
		t.Fatalf("memo = %q, want hello", got)
	}
	messages := body.Get(bodyFields.ByName("messages")).List()
	if messages.Len() != 1 {
		t.Fatalf("messages length = %d, want 1", messages.Len())
	}
	anyMsg := messages.Get(0).Message()
	if got := anyMsg.Get(anyMsg.Descriptor().Fields().ByName("type_url")).String(); got != "/test.v1.MsgSend" {
		t.Fatalf("type_url = %q, want /test.v1.MsgSend", got)
	}

	// AuthInfo uses SIGN_MODE_DIRECT and the secp256k1 public key.
	auth := decodeDynamic(t, resolver, authInfoType, tx.AuthInfoBytes)
	signerInfos := auth.Get(auth.Descriptor().Fields().ByName("signer_infos")).List()
	if signerInfos.Len() != 1 {
		t.Fatalf("signer_infos length = %d, want 1", signerInfos.Len())
	}
	info := signerInfos.Get(0).Message()
	pubKey := info.Get(info.Descriptor().Fields().ByName("public_key")).Message()
	if got := pubKey.Get(pubKey.Descriptor().Fields().ByName("type_url")).String(); got != secp256k1PubKeyTypeURL {
		t.Fatalf("public key type_url = %q, want %q", got, secp256k1PubKeyTypeURL)
	}
	modeInfo := info.Get(info.Descriptor().Fields().ByName("mode_info")).Message()
	single := modeInfo.Get(modeInfo.Descriptor().Fields().ByName("single")).Message()
	if got := single.Get(single.Descriptor().Fields().ByName("mode")).Enum(); got != 1 {
		t.Fatalf("sign mode = %d, want 1 (SIGN_MODE_DIRECT)", got)
	}
	if got := info.Get(info.Descriptor().Fields().ByName("sequence")).Uint(); got != 3 {
		t.Fatalf("sequence = %d, want 3", got)
	}

	// TxRaw must embed the exact body/auth bytes and the signature.
	txRaw := decodeDynamic(t, resolver, txRawType, tx.TxBytes)
	txRawFields := txRaw.Descriptor().Fields()
	if got := txRaw.Get(txRawFields.ByName("body_bytes")).Bytes(); string(got) != string(tx.BodyBytes) {
		t.Fatal("TxRaw.body_bytes does not match BodyBytes")
	}
	if got := txRaw.Get(txRawFields.ByName("auth_info_bytes")).Bytes(); string(got) != string(tx.AuthInfoBytes) {
		t.Fatal("TxRaw.auth_info_bytes does not match AuthInfoBytes")
	}
	signatures := txRaw.Get(txRawFields.ByName("signatures")).List()
	if signatures.Len() != 1 || string(signatures.Get(0).Bytes()) != string(tx.Signature) {
		t.Fatal("TxRaw.signatures does not match the signature")
	}

	// The embedded MsgSend carries its fields.
	anyValue := anyMsg.Get(anyMsg.Descriptor().Fields().ByName("value")).Bytes()
	msgSend := decodeDynamic(t, resolver, "test.v1.MsgSend", anyValue)
	msgFields := msgSend.Descriptor().Fields()
	if got := msgSend.Get(msgFields.ByName("from_address")).String(); got != "cosmos1aaa" {
		t.Fatalf("from_address = %q, want cosmos1aaa", got)
	}
	if got := msgSend.Get(msgFields.ByName("amount")).List().Len(); got != 1 {
		t.Fatalf("amount length = %d, want 1", got)
	}
}

func TestBuildAndSignRejectsUnknownMessageType(t *testing.T) {
	resolver := testResolver{testFiles(t)}
	signer, _ := NewPrivateKeySigner(privKeyOneHex, Secp256k1)
	_, err := BuildAndSign(resolver, signer, "test-chain-1", []Msg{Msg(`{"@type":"/test.v1.DoesNotExist"}`)}, TxOptions{})
	if err == nil {
		t.Fatal("unknown message type should fail")
	}
}

func TestBuildAndSignValidation(t *testing.T) {
	resolver := testResolver{testFiles(t)}
	signer, _ := NewPrivateKeySigner(privKeyOneHex, Secp256k1)
	msg := Msg(`{"@type":"/test.v1.MsgSend"}`)

	if _, err := BuildAndSign(resolver, signer, "", []Msg{msg}, TxOptions{}); err == nil {
		t.Fatal("empty chainID should fail")
	}
	if _, err := BuildAndSign(resolver, signer, "chain", nil, TxOptions{}); err == nil {
		t.Fatal("no messages should fail")
	}
	if _, err := BuildAndSign(resolver, signer, "chain", []Msg{Msg(`{`)}, TxOptions{}); err == nil {
		t.Fatal("invalid JSON should fail")
	}
	if _, err := BuildAndSign(resolver, nil, "chain", []Msg{msg}, TxOptions{}); err == nil {
		t.Fatal("nil signer should fail")
	}
}

func TestBuildAndSignEthSecp256k1(t *testing.T) {
	resolver := testResolver{testFiles(t)}
	signer, err := NewPrivateKeySigner(privKeyOneHex, EthSecp256k1)
	if err != nil {
		t.Fatalf("NewPrivateKeySigner failed: %v", err)
	}
	tx, err := BuildAndSign(resolver, signer, "chain-1", []Msg{Msg(`{"@type":"/test.v1.MsgSend"}`)}, TxOptions{})
	if err != nil {
		t.Fatalf("BuildAndSign failed: %v", err)
	}
	auth := decodeDynamic(t, resolver, authInfoType, tx.AuthInfoBytes)
	signerInfos := auth.Get(auth.Descriptor().Fields().ByName("signer_infos")).List()
	info := signerInfos.Get(0).Message()
	pubKey := info.Get(info.Descriptor().Fields().ByName("public_key")).Message()
	if got := pubKey.Get(pubKey.Descriptor().Fields().ByName("type_url")).String(); got != ethSecp256k1PubKeyURL {
		t.Fatalf("public key type_url = %q, want %q", got, ethSecp256k1PubKeyURL)
	}
}

func TestBuildAndSignRejectsBadFeeCoin(t *testing.T) {
	resolver := testResolver{testFiles(t)}
	signer, _ := NewPrivateKeySigner(privKeyOneHex, Secp256k1)
	_, err := BuildAndSign(resolver, signer, "chain", []Msg{Msg(`{"@type":"/test.v1.MsgSend"}`)}, TxOptions{
		Fee: []Coin{{Denom: "uatom"}},
	})
	if err == nil {
		t.Fatal("fee coin without amount should fail")
	}
}

func TestResolvePubKeyTypeURL(t *testing.T) {
	resolver := testResolver{testFiles(t)}
	url, err := ResolvePubKeyTypeURL(resolver, Secp256k1)
	if err != nil || url != secp256k1PubKeyTypeURL {
		t.Fatalf("secp url = %q, err = %v", url, err)
	}
	url, err = ResolvePubKeyTypeURL(resolver, EthSecp256k1)
	if err != nil || url != ethSecp256k1PubKeyURL {
		t.Fatalf("ethsecp url = %q, err = %v", url, err)
	}
	if _, err := ResolvePubKeyTypeURL(nil, Secp256k1); err == nil {
		t.Fatal("nil resolver should error")
	}
}

func TestPubKeyTypeURLOverride(t *testing.T) {
	if _, err := NewPrivateKeySigner(privKeyOneHex, Secp256k1, WithPubKeyTypeURL("/x.y.PubKey")); err == nil {
		t.Fatal("type URL override should be rejected for secp256k1")
	}
	signer, err := NewPrivateKeySigner(privKeyOneHex, EthSecp256k1, WithPubKeyTypeURL("/injective.crypto.v1beta1.ethsecp256k1.PubKey"))
	if err != nil {
		t.Fatalf("override for ethsecp256k1 failed: %v", err)
	}
	if signer.PubKeyTypeURL() != "/injective.crypto.v1beta1.ethsecp256k1.PubKey" {
		t.Fatalf("type URL = %q", signer.PubKeyTypeURL())
	}
}

func TestParseBroadcastResponse(t *testing.T) {
	resp, err := parseBroadcastResponse([]byte(`{"txResponse":{"code":0,"txhash":"ABC"}}`))
	if err != nil {
		t.Fatalf("success response should not error: %v", err)
	}
	if resp.Code != 0 || resp.TxHash != "ABC" {
		t.Fatalf("unexpected response %+v", resp)
	}

	logText := "insufficient fees"
	encoded := base64.StdEncoding.EncodeToString([]byte(logText))
	resp, err = parseBroadcastResponse([]byte(`{"tx_response":{"code":13,"codespace":"sdk","txhash":"DEF","raw_log":"` + encoded + `"}}`))
	if err == nil {
		t.Fatal("non-zero code should error")
	}
	if resp.Code != 13 || resp.Codespace != "sdk" {
		t.Fatalf("unexpected response %+v", resp)
	}
	if !strings.Contains(err.Error(), logText) {
		t.Fatalf("error %q should contain the decoded raw_log", err)
	}
}

func TestParseSimulateGas(t *testing.T) {
	gas, err := parseSimulateGas([]byte(`{"gasInfo":{"gasUsed":"12345"}}`))
	if err != nil || gas != 12345 {
		t.Fatalf("gas = %d, err = %v, want 12345", gas, err)
	}
	gas, err = parseSimulateGas([]byte(`{"gas_info":{"gas_used":7}}`))
	if err != nil || gas != 7 {
		t.Fatalf("gas = %d, err = %v, want 7", gas, err)
	}
	if _, err := parseSimulateGas([]byte(`{}`)); err == nil {
		t.Fatal("missing gasInfo should error")
	}
}

func TestParseAccountSequence(t *testing.T) {
	number, sequence, err := parseAccountSequence([]byte(`{"baseAccount":{"accountNumber":"7","sequence":"3"}}`))
	if err != nil || number != 7 || sequence != 3 {
		t.Fatalf("number=%d sequence=%d err=%v", number, sequence, err)
	}
	number, sequence, err = parseAccountSequence([]byte(`{"accountNumber":"1","sequence":"0"}`))
	if err != nil || number != 1 || sequence != 0 {
		t.Fatalf("number=%d sequence=%d err=%v, want 1/0", number, sequence, err)
	}
	if _, _, err := parseAccountSequence([]byte(`{"unrelated":"x"}`)); err == nil {
		t.Fatal("missing fields should error")
	}
}

func decodeDynamic(t *testing.T, resolver Resolver, name string, data []byte) *dynamicpb.Message {
	t.Helper()
	msgType, err := resolver.FindMessageByName(protoreflect.FullName(name))
	if err != nil {
		t.Fatalf("resolve %s: %v", name, err)
	}
	msg := dynamicpb.NewMessage(msgType.Descriptor())
	if err := proto.Unmarshal(data, msg); err != nil {
		t.Fatalf("unmarshal %s: %v", name, err)
	}
	return msg
}

// testResolver is a minimal Resolver backed by a protoregistry.Files.
type testResolver struct {
	files *protoregistry.Files
}

func (r testResolver) FindMessageByName(name protoreflect.FullName) (protoreflect.MessageType, error) {
	desc, err := r.files.FindDescriptorByName(name)
	if err != nil {
		return nil, err
	}
	md, ok := desc.(protoreflect.MessageDescriptor)
	if !ok {
		return nil, protoregistry.NotFound
	}
	return dynamicpb.NewMessageType(md), nil
}

func (r testResolver) FindMessageByURL(url string) (protoreflect.MessageType, error) {
	name := url
	if idx := strings.LastIndex(url, "/"); idx >= 0 {
		name = url[idx+1:]
	}
	return r.FindMessageByName(protoreflect.FullName(name))
}

func (r testResolver) FindExtensionByName(protoreflect.FullName) (protoreflect.ExtensionType, error) {
	return nil, protoregistry.NotFound
}

func (r testResolver) FindExtensionByNumber(protoreflect.FullName, protoreflect.FieldNumber) (protoreflect.ExtensionType, error) {
	return nil, protoregistry.NotFound
}

func testFiles(t *testing.T) *protoregistry.Files {
	t.Helper()
	files := &protoregistry.Files{}

	anyFD, err := protoregistry.GlobalFiles.FindFileByPath("google/protobuf/any.proto")
	if err != nil {
		t.Fatalf("find any.proto: %v", err)
	}
	if err := files.RegisterFile(anyFD); err != nil {
		t.Fatalf("register any.proto: %v", err)
	}

	register(t, files, coinFile())
	register(t, files, signingFile())
	register(t, files, secp256k1File())
	register(t, files, ethSecp256k1File())
	register(t, files, txFile())
	register(t, files, testMsgFile())
	return files
}

func register(t *testing.T, files *protoregistry.Files, fd *descriptorpb.FileDescriptorProto) {
	t.Helper()
	built, err := protodesc.NewFile(fd, files)
	if err != nil {
		t.Fatalf("build %s: %v", fd.GetName(), err)
	}
	if err := files.RegisterFile(built); err != nil {
		t.Fatalf("register %s: %v", fd.GetName(), err)
	}
}

func coinFile() *descriptorpb.FileDescriptorProto {
	return &descriptorpb.FileDescriptorProto{
		Name:    proto.String("cosmos/base/v1beta1/coin.proto"),
		Package: proto.String("cosmos.base.v1beta1"),
		Syntax:  proto.String("proto3"),
		MessageType: []*descriptorpb.DescriptorProto{
			{Name: proto.String("Coin"), Field: []*descriptorpb.FieldDescriptorProto{
				protoStringField("denom", 1),
				protoStringField("amount", 2),
			}},
		},
	}
}

func signingFile() *descriptorpb.FileDescriptorProto {
	return &descriptorpb.FileDescriptorProto{
		Name:    proto.String("cosmos/tx/signing/v1beta1/signing.proto"),
		Package: proto.String("cosmos.tx.signing.v1beta1"),
		Syntax:  proto.String("proto3"),
		EnumType: []*descriptorpb.EnumDescriptorProto{
			{
				Name: proto.String("SignMode"),
				Value: []*descriptorpb.EnumValueDescriptorProto{
					{Name: proto.String("SIGN_MODE_UNSPECIFIED"), Number: proto.Int32(0)},
					{Name: proto.String("SIGN_MODE_DIRECT"), Number: proto.Int32(1)},
				},
			},
		},
	}
}

func secp256k1File() *descriptorpb.FileDescriptorProto {
	return &descriptorpb.FileDescriptorProto{
		Name:    proto.String("cosmos/crypto/secp256k1/keys.proto"),
		Package: proto.String("cosmos.crypto.secp256k1"),
		Syntax:  proto.String("proto3"),
		MessageType: []*descriptorpb.DescriptorProto{
			{Name: proto.String("PubKey"), Field: []*descriptorpb.FieldDescriptorProto{
				protoBytesField("key", 1, false),
			}},
		},
	}
}

func ethSecp256k1File() *descriptorpb.FileDescriptorProto {
	return &descriptorpb.FileDescriptorProto{
		Name:    proto.String("ethermint/crypto/v1/ethsecp256k1/keys.proto"),
		Package: proto.String("ethermint.crypto.v1.ethsecp256k1"),
		Syntax:  proto.String("proto3"),
		MessageType: []*descriptorpb.DescriptorProto{
			{Name: proto.String("PubKey"), Field: []*descriptorpb.FieldDescriptorProto{
				protoBytesField("key", 1, false),
			}},
		},
	}
}

func txFile() *descriptorpb.FileDescriptorProto {
	modeInfo := &descriptorpb.DescriptorProto{
		Name: proto.String("ModeInfo"),
		OneofDecl: []*descriptorpb.OneofDescriptorProto{
			{Name: proto.String("sum")},
		},
		Field: []*descriptorpb.FieldDescriptorProto{
			{
				Name:       proto.String("single"),
				Number:     proto.Int32(1),
				Type:       descriptorpb.FieldDescriptorProto_TYPE_MESSAGE.Enum(),
				TypeName:   proto.String(".cosmos.tx.v1beta1.ModeInfo.Single"),
				Label:      descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL.Enum(),
				OneofIndex: proto.Int32(0),
			},
		},
		NestedType: []*descriptorpb.DescriptorProto{
			{Name: proto.String("Single"), Field: []*descriptorpb.FieldDescriptorProto{
				{
					Name:     proto.String("mode"),
					Number:   proto.Int32(1),
					Type:     descriptorpb.FieldDescriptorProto_TYPE_ENUM.Enum(),
					TypeName: proto.String(".cosmos.tx.signing.v1beta1.SignMode"),
					Label:    descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL.Enum(),
				},
			}},
		},
	}
	return &descriptorpb.FileDescriptorProto{
		Name:    proto.String("cosmos/tx/v1beta1/tx.proto"),
		Package: proto.String("cosmos.tx.v1beta1"),
		Syntax:  proto.String("proto3"),
		Dependency: []string{
			"google/protobuf/any.proto",
			"cosmos/base/v1beta1/coin.proto",
			"cosmos/tx/signing/v1beta1/signing.proto",
		},
		MessageType: []*descriptorpb.DescriptorProto{
			{
				Name: proto.String("TxBody"),
				Field: []*descriptorpb.FieldDescriptorProto{
					protoMessageField("messages", 1, ".google.protobuf.Any", true),
					protoStringField("memo", 2),
					protoUint64Field("timeout_height", 3),
				},
			},
			{
				Name: proto.String("AuthInfo"),
				Field: []*descriptorpb.FieldDescriptorProto{
					protoMessageField("signer_infos", 1, ".cosmos.tx.v1beta1.SignerInfo", true),
					protoMessageField("fee", 2, ".cosmos.tx.v1beta1.Fee", false),
				},
			},
			{
				Name: proto.String("SignerInfo"),
				Field: []*descriptorpb.FieldDescriptorProto{
					protoMessageField("public_key", 1, ".google.protobuf.Any", false),
					protoMessageField("mode_info", 2, ".cosmos.tx.v1beta1.ModeInfo", false),
					protoUint64Field("sequence", 3),
				},
			},
			{
				Name: proto.String("Fee"),
				Field: []*descriptorpb.FieldDescriptorProto{
					protoMessageField("amount", 1, ".cosmos.base.v1beta1.Coin", true),
					protoUint64Field("gas_limit", 2),
					protoStringField("payer", 3),
					protoStringField("granter", 4),
				},
			},
			modeInfo,
			{
				Name: proto.String("SignDoc"),
				Field: []*descriptorpb.FieldDescriptorProto{
					protoBytesField("body_bytes", 1, false),
					protoBytesField("auth_info_bytes", 2, false),
					protoStringField("chain_id", 3),
					protoUint64Field("account_number", 4),
				},
			},
			{
				Name: proto.String("TxRaw"),
				Field: []*descriptorpb.FieldDescriptorProto{
					protoBytesField("body_bytes", 1, false),
					protoBytesField("auth_info_bytes", 2, false),
					protoBytesField("signatures", 3, true),
				},
			},
		},
	}
}

func testMsgFile() *descriptorpb.FileDescriptorProto {
	return &descriptorpb.FileDescriptorProto{
		Name:       proto.String("test/v1/tx.proto"),
		Package:    proto.String("test.v1"),
		Syntax:     proto.String("proto3"),
		Dependency: []string{"cosmos/base/v1beta1/coin.proto"},
		MessageType: []*descriptorpb.DescriptorProto{
			{
				Name: proto.String("MsgSend"),
				Field: []*descriptorpb.FieldDescriptorProto{
					protoStringField("from_address", 1),
					protoStringField("to_address", 2),
					protoMessageField("amount", 3, ".cosmos.base.v1beta1.Coin", true),
				},
			},
		},
	}
}

func protoStringField(name string, num int32) *descriptorpb.FieldDescriptorProto {
	return &descriptorpb.FieldDescriptorProto{
		Name:   proto.String(name),
		Number: proto.Int32(num),
		Type:   descriptorpb.FieldDescriptorProto_TYPE_STRING.Enum(),
		Label:  descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL.Enum(),
	}
}

func protoBytesField(name string, num int32, repeated bool) *descriptorpb.FieldDescriptorProto {
	label := descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL
	if repeated {
		label = descriptorpb.FieldDescriptorProto_LABEL_REPEATED
	}
	return &descriptorpb.FieldDescriptorProto{
		Name:   proto.String(name),
		Number: proto.Int32(num),
		Type:   descriptorpb.FieldDescriptorProto_TYPE_BYTES.Enum(),
		Label:  label.Enum(),
	}
}

func protoUint64Field(name string, num int32) *descriptorpb.FieldDescriptorProto {
	return &descriptorpb.FieldDescriptorProto{
		Name:   proto.String(name),
		Number: proto.Int32(num),
		Type:   descriptorpb.FieldDescriptorProto_TYPE_UINT64.Enum(),
		Label:  descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL.Enum(),
	}
}

func protoMessageField(name string, num int32, typeName string, repeated bool) *descriptorpb.FieldDescriptorProto {
	label := descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL
	if repeated {
		label = descriptorpb.FieldDescriptorProto_LABEL_REPEATED
	}
	return &descriptorpb.FieldDescriptorProto{
		Name:     proto.String(name),
		Number:   proto.Int32(num),
		Type:     descriptorpb.FieldDescriptorProto_TYPE_MESSAGE.Enum(),
		TypeName: proto.String(typeName),
		Label:    label.Enum(),
	}
}
