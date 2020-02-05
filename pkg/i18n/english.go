package i18n

// TranslationSet is a set of localised strings for a given language
type TranslationSet struct {
	AddFavourite string
	ErrorMessage string
}

func englishSet() TranslationSet {
	return TranslationSet{
		AddFavourite: "Add favourite",
		ErrorMessage: "Error Message",
	}
}
