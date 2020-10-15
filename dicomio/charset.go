package dicomio

import (
	"fmt"

	"github.com/hxhxhx88/go-dicom/dicomlog"
	"golang.org/x/text/encoding"
	"golang.org/x/text/encoding/htmlindex"
)

// CodingSystem defines how a []byte is translated into a utf8 string.
type CodingSystem struct {
	// VR="PN" is the only place where we potentially use all three
	// decoders.  For all other VR types, only Ideographic decoder is used.
	// See P3.5, 6.2.
	//
	// P3.5 6.1 is supposed to define the coding systems in detail.  But the
	// spec text is insanely obtuse and I couldn't tell what its meaning
	// after hours of trying. So I just copied what pydicom charset.py is
	// doing.
	Alphabetic  *encoding.Decoder
	Ideographic *encoding.Decoder
	Phonetic    *encoding.Decoder
}

// CodingSystemType defines the where the coding system is going to be
// used. This distinction is useful in Japanese, but of little use in other
// languages.
type CodingSystemType int

const (
	// AlphabeticCodingSystem is for writing a name in (English) alphabets.
	AlphabeticCodingSystem CodingSystemType = iota
	// IdeographicCodingSystem is for writing the name in the native writing
	// system (Kanji).
	IdeographicCodingSystem
	// PhoneticCodingSystem is for hirakana and/or katakana.
	PhoneticCodingSystem
)

// Mapping of DICOM charset name to golang encoding/htmlindex name.  "" means
// 7bit ascii.
var htmlEncodingNames = map[string]string{
	"ISO 2022 IR 6":   "iso-8859-1",
	"ISO_IR 13":       "shift_jis",
	"ISO 2022 IR 13":  "shift_jis",
	"ISO_IR 100":      "iso-8859-1",
	"ISO 2022 IR 100": "iso-8859-1",
	"ISO_IR 101":      "iso-8859-2",
	"ISO 2022 IR 101": "iso-8859-2",
	"ISO_IR 109":      "iso-8859-3",
	"ISO 2022 IR 109": "iso-8859-3",
	"ISO_IR 110":      "iso-8859-4",
	"ISO 2022 IR 110": "iso-8859-4",
	"ISO_IR 126":      "iso-ir-126",
	"ISO 2022 IR 126": "iso-ir-126",
	"ISO_IR 127":      "iso-ir-127",
	"ISO 2022 IR 127": "iso-ir-127",
	"ISO_IR 138":      "iso-ir-138",
	"ISO 2022 IR 138": "iso-ir-138",
	"ISO_IR 144":      "iso-ir-144",
	"ISO 2022 IR 144": "iso-ir-144",
	"ISO_IR 148":      "iso-ir-148",
	"ISO 2022 IR 148": "iso-ir-148",
	"ISO 2022 IR 149": "euc-kr",
	"ISO 2022 IR 159": "iso-2022-jp",
	"ISO_IR 166":      "iso-ir-166",
	"ISO 2022 IR 166": "iso-ir-166",
	"ISO 2022 IR 87":  "iso-2022-jp",
	"GB18030":         "gb18030",
	"GBK":             "gbk",
	"ISO_IR 192":      "utf-8",

	// Encoding `ISO 2022 IR 58` is suggested to map to `ISO-2022-CN` at
	// 	 http://dicom.nema.org/medical/dicom/current/output/chtml/part18/chapter_D.html
	// However, `ISO-2022-CN` encoding is a deprecated encoding for HTML client, see
	//   https://developer.mozilla.org/en-US/docs/Web/API/TextDecoder/TextDecoder
	//   https://developer.mozilla.org/en-US/docs/Web/API/TextDecoder/encoding
	// and I quote from https://en.wikipedia.org/wiki/ISO/IEC_2022#Other_7-bit_versions
	//   ```
	//   ISO-2022-KR and ISO-2022-CN are used less frequently than ISO-2022-JP, and are sometimes deliberately not supported due to security concerns.
	//   Notably, the WHATWG Encoding Standard used by HTML5 maps ISO-2022-KR, ISO-2022-CN and ISO-2022-CN-EXT (as well as HZ-GB-2312) to the "replacement" decoder
	//     which maps all input to the replacement character (�), in order to prevent certain cross-site scripting and related attacks,
	//     which utilize a difference in encoding support between the client and server.
	//   Although the same security concern (allowing sequences of ASCII bytes to be interpreted differently) also applies to ISO-2022-JP and UTF-16,
	//     they could not be given this treatment due to being much more frequently used in deployed content.
	//   ```
	// and in turn GoLang maps through `htmlindex.Get` to a `Replacement` encoder, see
	//   https://github.com/golang/text/blob/release-branch.go1.15/encoding/htmlindex/tables.go#L306
	// which replaces whatever codes into a fixed replacement character �, i.e. three bytes [239, 191, 189], see
	//   https://github.com/golang/text/blob/release-branch.go1.15/encoding/encoding.go#L167
	// However, it is also suggested to map DICOM `ISO-IR 58` to `GB2312`, see
	//   http://dicom.nema.org/medical/dicom/current/output/chtml/part05/chapter_K.html
	// I am no sure if `ISO 2022 IR 58` and `ISO-IR 58` are the same thing, but what else can I do.
	// If `ISO 2022 IR 58` is mapped to the `replacement` decoder, all metadata following `SpecificCharacterSet` metadata will be parsed to �,
	//   and if unfortunately some important data, like InstanceNumber, are among the followings, bad things may happen.
	"ISO 2022 IR 58": "gb2312",
}

// ParseSpecificCharacterSet converts DICOM character encoding names, such as
// "ISO-IR 100" to golang decoder. It will return nil, nil for the default (7bit
// ASCII) encoding. Cf. P3.2
// D.6.2. http://dicom.nema.org/medical/dicom/2016d/output/chtml/part02/sect_D.6.2.html
func ParseSpecificCharacterSet(encodingNames []string) (CodingSystem, error) {
	// Set the []byte -> string decoder for the rest of the
	// file.  It's sad that SpecificCharacterSet isn't part
	// of metadata, but is part of regular attrs, so we need
	// to watch out for multiple occurrences of this type of
	// elements.
	// encodingNames, err := elem.GetStrings()
	//if err != nil {
	//return CodingSystem{}, err
	//}
	var decoders []*encoding.Decoder
	for _, name := range encodingNames {
		var c *encoding.Decoder
		dicomlog.Vprintf(2, "dicom.ParseSpecificCharacterSet: Using coding system %s", name)
		if htmlName, ok := htmlEncodingNames[name]; !ok {
			// TODO(saito) Support more encodings.
			return CodingSystem{}, fmt.Errorf("dicom.ParseSpecificCharacterSet: Unknown character set '%s'. Assuming utf-8", name)
		} else {
			if htmlName != "" {
				d, err := htmlindex.Get(htmlName)
				if err != nil {
					panic(fmt.Sprintf("Encoding name %s (for %s) not found", name, htmlName))
				}
				c = d.NewDecoder()
			}
		}
		decoders = append(decoders, c)
	}
	if len(decoders) == 0 {
		return CodingSystem{nil, nil, nil}, nil
	}
	if len(decoders) == 1 {
		return CodingSystem{decoders[0], decoders[0], decoders[0]}, nil
	}
	if len(decoders) == 2 {
		return CodingSystem{decoders[0], decoders[1], decoders[1]}, nil
	}
	return CodingSystem{decoders[0], decoders[1], decoders[2]}, nil
}
