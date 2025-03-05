package EbayBookBuilder

import (
	"Scraper/DataTypes"
	"Scraper/Ebay"
	"Scraper/MongoDBInteractions"
	"fmt"
	"math"
	"os"
	"strconv"
	"strings"
)

func getCategorySwitch(categoryName string) DataTypes.Category {
	switch categoryName {
	case "Sport":
		return DataTypes.Saggistica
	case "Fumetti":
		return DataTypes.Saggistica
	case "Psicologia e Filosofia":
		return DataTypes.Saggistica
	case "Politica e Società":
		return DataTypes.Saggistica
	case "Passione e Sentimenti":
		return DataTypes.Narrativa
	case "Economia Diritto e Lavoro":
		return DataTypes.Saggistica
	case "Ambiente e Animali":
		return DataTypes.Saggistica
	case "Hobby e Tempo libero":
		return DataTypes.Saggistica
	case "Esoterismo e Astrologia":
		return DataTypes.Saggistica
	case "Architettura Design e Moda":
		return DataTypes.Saggistica
	case "Gastronomia":
		return DataTypes.CucinaEGastronomia
	case "Scienza e Tecnica":
		return DataTypes.Saggistica
	case "Bambini e Ragazzi":
		return DataTypes.BambiniERagazzi
	case "Cinema e Spettacolo":
		return DataTypes.Saggistica
	case "Arte Beni culturali e Fotografia":
		return DataTypes.Saggistica
	case "Fantasy Horror e Gothic":
		return DataTypes.Narrativa
	case "Storia e Biografie":
		return DataTypes.Saggistica
	case "Lingue e Dizionari":
		return DataTypes.CorsiDiLingua
	case "Salute Benessere Self Help":
		return DataTypes.Saggistica
	case "Gialli Noir e Avventura":
		return DataTypes.Narrativa
	case "Romanzi e Letterature":
		return DataTypes.Narrativa
	case "Guide turistiche e Viaggi":
		return DataTypes.Saggistica
	case "Informatica e Web":
		return DataTypes.Saggistica
	case "Musica":
		return DataTypes.Saggistica
	case "Religioni e Spiritualità":
		return DataTypes.Saggistica
	case "Famiglia Scuola e Università":
		return DataTypes.LibriDiTesto
	default:
		_, _ = fmt.Fprintf(os.Stderr, "Didn't find a category id for %s\n", categoryName)
		return DataTypes.Saggistica
	}
}

// Function to get the category ID based on the input string
func getCategoryIDMondadori(category string) DataTypes.Category {
	// Split the input string and get the last part
	splitCategories := strings.Split(category, "/")
	category = splitCategories[len(splitCategories)-1]

	return getCategorySwitch(category)
}

func getFinalPrice(originalPriceString string, competitionPrice float64) string {
	// Convert string to float64
	originalPrice, err := strconv.ParseFloat(originalPriceString, 64)
	if err != nil {
		panic(err)
	}
	finalPrice := (math.Max(originalPrice*0.1, 1) + originalPrice + 0.43) / 0.866
	minPrice := (originalPrice + 0.10 + 0.43) / 0.866

	// Round to fixed price
	if finalPrice-float64(int(finalPrice)) > 0.49 {
		finalPrice = float64(int(finalPrice)) + 0.99
	} else {
		finalPrice = float64(int(finalPrice)) + 0.49
	}

	if minPrice < competitionPrice && competitionPrice < finalPrice {
		finalPrice = competitionPrice - 0.01
	}

	return fmt.Sprintf("%.2f", math.Round(finalPrice*100)/100)
}

func getFinalTitle(title string, author string, category string, language ...string) string {
	// Remove unwanted characters
	title = strings.ReplaceAll(title, ".", "")
	title = strings.ReplaceAll(title, ":", "")
	title = strings.ReplaceAll(title, "vol", "volume")

	// Split title into words
	words := strings.Fields(title)

	// Define articles
	articles := map[string]bool{
		"il": true, "lo": true, "la": true, "i": true, "gli": true, "le": true,
		"un": true, "uno": true, "una": true, "di": true, "a": true, "da": true,
		"in": true, "con": true, "su": true, "per": true, "tra": true, "fra": true,
	}

	// Capitalize words except articles
	for i := 1; i < len(words); i++ {
		if articles[strings.ToLower(words[i])] {
			words[i] = strings.ToLower(words[i])
		} else {
			words[i] = strings.Title(strings.ToLower(words[i]))
		}
	}

	// Join words back into title
	title = strings.Join(words, " ")

	// Truncate title if it exceeds 80 characters
	if len(title) > 80 {
		title = title[:80]
	}

	// Clean up author name
	author = strings.ReplaceAll(author, "  ", " ")

	// Append author to title if space allows
	if len(title)+len(author)+3 <= 80 {
		title = title + " - " + author
	} else if len(title)+len(author)+1 <= 80 {
		title = title + " " + author
	}

	// Extract the last part of the category
	category = strings.Split(category, "/")[len(strings.Split(category, "/"))-1]

	// Append category-specific prefix
	if category == "Fumetti" {
		if len(title)+16 <= 80 {
			title = "Fumetto NUOVO - " + title
		} else if len(title)+14 <= 80 {
			title = "Fumetto NUOVO " + title
		} else if len(title)+8 <= 80 {
			title = "Fumetto " + title
		}
	} else {
		if len(title)+14 <= 80 {
			title = "Libro NUOVO - " + title
		} else if len(title)+12 <= 80 {
			title = "Libro NUOVO " + title
		} else if len(title)+6 <= 80 {
			title = "Libro " + title
		}
	}

	// Append language if provided
	if len(language) > 0 && language[0] != "" {
		lang := language[0]
		if len(title)+4+len(lang) <= 80 {
			title = title + " in " + lang
		} else if len(title)+1+len(lang) <= 80 {
			title = title + " " + lang
		}
	}

	return title
}

func BuildEbayBooks(isbns []string) map[string]DataTypes.EbayBook {
	ebayBooks := make(map[string]DataTypes.EbayBook)
	for _, isbn := range isbns {
		ebayBook := BuildEbayBook(isbn)
		if ebayBook != nil {
			ebayBooks[isbn] = *ebayBook
		}
	}
	return ebayBooks
}

func BuildEbayBook(isbn string) *DataTypes.EbayBook {
	mondadoriBook, err := MongoDBInteractions.GetMondadoriBook(isbn)
	if err != nil {
		return nil
	}
	categoryID := getCategoryIDMondadori(mondadoriBook.Categories[0])
	available := mondadoriBook.Available == "Disponibilità immediata"
	competitionPrice := Ebay.SearchMinCost(isbn)
	price := getFinalPrice(mondadoriBook.Price, competitionPrice)
	title := getFinalTitle(mondadoriBook.Title, mondadoriBook.Author, mondadoriBook.Categories[0])
	ebayBook := DataTypes.EbayBook{ISBN: isbn,
		CategoryID:    categoryID,
		Available:     available,
		Editor:        mondadoriBook.Editor,
		Author:        mondadoriBook.Author,
		Variant:       mondadoriBook.Variant,
		Language:      mondadoriBook.Language,
		Price:         price,
		Title:         title,
		PublishedFrom: mondadoriBook.PublishedFrom,
		Pages:         mondadoriBook.Pages,
		MarketIn:      "Mondadori",
	}
	return &ebayBook
}
