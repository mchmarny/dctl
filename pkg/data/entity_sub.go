package data

var (
	entityNoise = map[string]bool{
		"B.V.":        true,
		"CDL":         true,
		"CO":          true,
		"COMPANY":     true,
		"CORP":        true,
		"CORPORATION": true,
		"GMBH":        true,
		"GROUP":       true,
		"INC":         true,
		"LLC":         true,
		"L.L.C.":      true,
		"L.C.":        true,
		"LC":          true,
		"P.C.":        true,
		"P.A.":        true,
		"S.C.":        true,
		"LTD.":        true,
		"CHTD.":       true,
		"PC":          true,
		"LTD":         true,
		"PVT":         true,
		"SE":          true,
		"S.A.":        true,
		"S.C.A.":      true,
		"S.C.O.":      true,
		"S.C.P.":      true,
		"S.C.S.":      true,
		"S.C.V.":      true,
	}

	entitySubstitutions = map[string]string{
		"CHAINGUARDDEV":       "CHAINGUARD",
		"GCP":                 "GOOGLE",
		"GOOGLECLOUD":         "GOOGLE",
		"GOOGLECLOUDPLATFORM": "GOOGLE",
		"HUAWEICLOUD":         "HUAWEI",
		"IBM CODAITY":         "IBM",
		"IBM RESEARCH":        "IBM",
		"INTERNATIONAL BUSINESS MACHINES CORPORATION":                 "IBM",
		"INTERNATIONAL BUSINESS MACHINES":                             "IBM",
		"INTERNATIONAL INSTITUTE OF INFORMATION TECHNOLOGY BANGALORE": "IIIT BANGALORE",
		"LINE PLUS":       "LINE",
		"MICROSOFT CHINA": "MICROSOFT",
		"REDHATOFFICIAL":  "REDHAT",
		"S&P GLOBAL INC":  "S&P",
		"S&P GLOBAL":      "S&P",
		"VERVERICA ORIGINAL CREATORS OF APACHE FLINK": "VERVERICA",
	}
)
