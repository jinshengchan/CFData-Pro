package main

import "fmt"

const bannerLogo = "ＣＦＤＡＴＡ-ＰＲＯ"
const bannerAuthor = "by:GitHub/jinshengchan"

func printBanner() {
	fmt.Println()
	fmt.Println(colorize(bannerLogo, ansiBold+ansiGreen))
	fmt.Println(bannerAuthor)
	fmt.Println()
}
