package PriceConversion

import (
	"strconv"
	"strings"
)

func ConvertPriceToFloat(priceStr string) float64 {
	priceStr = strings.ReplaceAll(priceStr, ",", ".")
	price, err := strconv.ParseFloat(priceStr, 64)
	if err != nil {
		panic(err)
	}
	return price
}
