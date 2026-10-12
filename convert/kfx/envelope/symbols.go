// Package envelope encodes and inspects unencrypted, paged KFX envelopes.
// It treats the decoded book as an opaque byte stream.
package envelope

import "github.com/amazon-ion/ion-go/ion"

const (
	Magic          = "\xeaDRMION\xee"
	dictionarySize = 8 * 1024 * 1024
	protectedData  = "com.amazon.drm.ProtectedData@2.0"
	envelopeType   = "com.amazon.drm.Envelope@2.0"
	metadataType   = "com.amazon.drm.EnvelopeMetadata@2.0"
	plaintextType  = "com.amazon.drm.PlainText@2.0"
	compressedType = "com.amazon.drm.Compressed@1.0"
	indexType      = "com.amazon.drm.EnvelopeIndexTable@1.0"
)

func symbolTable() ion.SharedSymbolTable {
	return ion.NewSharedSymbolTable("ProtectedData", 1, []string{
		"com.amazon.drm.Envelope@1.0", "com.amazon.drm.EnvelopeMetadata@1.0", "size", "page_size",
		"encryption_key", "encryption_transformation", "encryption_voucher", "signing_key",
		"signing_algorithm", "signing_voucher", "com.amazon.drm.EncryptedPage@1.0", "cipher_text",
		"cipher_iv", "com.amazon.drm.Signature@1.0", "data", "com.amazon.drm.EnvelopeIndexTable@1.0",
		"length", "offset", "algorithm", "encoded", "encryption_algorithm", "hashing_algorithm",
		"expires", "format", "id", "lock_parameters", "strategy", "com.amazon.drm.Key@1.0",
		"com.amazon.drm.KeySet@1.0", "com.amazon.drm.PIDv3@1.0", "com.amazon.drm.PlainTextPage@1.0",
		"com.amazon.drm.PlainText@1.0", "com.amazon.drm.PrivateKey@1.0", "com.amazon.drm.PublicKey@1.0",
		"com.amazon.drm.SecretKey@1.0", "com.amazon.drm.Voucher@1.0", "public_key", "private_key",
		"com.amazon.drm.KeyPair@1.0", "com.amazon.drm.ProtectedData@1.0", "doctype",
		"com.amazon.drm.EnvelopeIndexTableOffset@1.0", "enddoc", "license_type", "license", "watermark",
		"key", "value", "com.amazon.drm.License@1.0", "category", "metadata", "categorized_metadata",
		"com.amazon.drm.CategorizedMetadata@1.0", "com.amazon.drm.VoucherEnvelope@1.0", "mac", "voucher",
		protectedData, envelopeType, metadataType, "com.amazon.drm.EncryptedPage@2.0",
		plaintextType, "compression_algorithm", compressedType, "page_index_table",
	})
}
