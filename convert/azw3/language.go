package azw3

import "golang.org/x/text/language"

// MOBI's legacy language word uses Windows primary/sublanguage numbers. EXTH 524
// separately preserves the full BCP-47 tag, including tags with no legacy code.
func mobiLanguage(tag language.Tag) uint32 {
	base, script, region := tag.Raw()
	primary := map[string]uint32{
		"af": 54, "sq": 28, "ar": 1, "hy": 43, "as": 77, "az": 44, "eu": 45, "be": 35, "bn": 69,
		"bg": 2, "ca": 3, "zh": 4, "hr": 26, "sr": 26, "cs": 5, "da": 6, "nl": 19, "en": 9, "es": 10,
		"et": 37, "fo": 56, "fa": 41, "fi": 11, "fr": 12, "ka": 55, "de": 7, "el": 8, "gu": 71,
		"he": 13, "hi": 57, "hu": 14, "is": 15, "id": 33, "it": 16, "ja": 17, "kn": 75, "kk": 63,
		"kok": 87, "ko": 18, "lv": 38, "lt": 39, "mk": 47, "ms": 62, "ml": 76, "mt": 58, "mr": 78,
		"ne": 97, "no": 20, "nb": 20, "nn": 20, "or": 72, "pl": 21, "pt": 22, "pa": 70, "rm": 23,
		"ro": 24, "ru": 25, "se": 59, "sa": 79, "sk": 27, "sl": 36, "sw": 65, "sv": 29, "ta": 73,
		"tt": 68, "te": 74, "th": 30, "ts": 49, "tn": 50, "tr": 31, "uk": 34, "ur": 32, "uz": 67,
		"vi": 42, "xh": 52, "zu": 53,
	}[base.String()]
	if primary == 0 {
		return 0
	}
	sublanguage := map[string]uint32{
		"en-US": 1, "en-GB": 2, "en-AU": 3, "en-CA": 4, "en-NZ": 5, "en-IE": 6, "en-ZA": 7,
		"en-JM": 8, "en-BZ": 10, "en-TT": 11, "en-ZW": 12, "en-PH": 13,
		"fr-FR": 1, "fr-BE": 2, "fr-CA": 3, "fr-CH": 4, "fr-LU": 5, "fr-MC": 6,
		"de-DE": 1, "de-CH": 2, "de-AT": 3, "de-LU": 4, "de-LI": 5,
		"zh-TW": 1, "zh-CN": 2, "zh-HK": 3, "zh-SG": 4, "pt-BR": 1, "pt-PT": 2,
		"it-IT": 1, "it-CH": 2, "nl-NL": 1, "nl-BE": 2, "sv-SE": 1, "sv-FI": 2,
		"ar-SA": 1, "ar-IQ": 2, "ar-EG": 3, "ar-LY": 4, "ar-DZ": 5, "ar-MA": 6, "ar-TN": 7,
		"ar-OM": 8, "ar-YE": 9, "ar-SY": 10, "ar-JO": 11, "ar-LB": 12, "ar-KW": 13, "ar-AE": 14,
		"ar-BH": 15, "ar-QA": 16, "es-MX": 2, "es-GT": 4, "es-CR": 5, "es-PA": 6, "es-DO": 7,
		"es-VE": 8, "es-CO": 9, "es-PE": 10, "es-AR": 11, "es-EC": 12, "es-CL": 13, "es-UY": 14,
		"es-PY": 15, "es-BO": 16, "es-SV": 17, "es-HN": 18, "es-NI": 19, "es-PR": 20,
	}[base.String()+"-"+region.String()]
	if base.String() == "sr" {
		sublanguage = 3
		if script.String() == "Latn" {
			sublanguage = 2
		}
	} else if base.String() == "nn" {
		sublanguage = 2
	} else if base.String() == "zh" && sublanguage == 0 {
		switch script.String() {
		case "Hant":
			sublanguage = 1
		case "Hans":
			sublanguage = 2
		}
	}
	return primary | sublanguage<<10
}
