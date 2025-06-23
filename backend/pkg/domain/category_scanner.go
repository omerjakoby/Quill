package domain

import (
	"strings"

	"github.com/cloudflare/ahocorasick"
)

// EmailCategories holds the keywords for each category.
// (This map is assumed to be provided and remains unchanged)
var EmailCategories = map[string]map[string]bool{
	"forums": {
		"discussion": true, "mailing list": true, "group message": true, "forum post": true,
		"thread": true, "digest": true, "newsletter": true, "community": true,
		"subscribe": true, "unsubscribe": true, "forum": true, "group": true,
		"list": true, "post": true, "topic": true, "conversation": true,
		"message board": true, "online community": true, "newsgroup": true, "mailing list archive": true,
		"replies": true, "thread summary": true, "recent posts": true, "discussion group": true,
		"subscribe to": true, "unsubscribe from": true, "usergroup": true, "new message in": true,
		"posted in": true, "re:": true, "fwd:": true, // Common email prefixes in forums/lists
		"google groups": true, "yahoo groups": true, "stack exchange": true, "quora digest": true,
		"reddit": true, "subscribers": true, "members": true, "moderator": true,
		"admin message": true, "announcement": true, "your subscription": true, "new reply": true,
		"new topic": true, "community discussion": true, "listserver": true,
	},
	"promotions": {
		"deal": true, "discount": true, "offer": true, "sale": true,
		"promotion": true, "coupon": true, "limited time": true, "save now": true,
		"special offer": true, "exclusive": true, "buy now": true, "shop now": true,
		"free shipping": true, "clearance": true, "new arrivals": true, "bargain": true,
		"discount code": true, "flash sale": true, "big savings": true, "seasonal sale": true,
		"cyber monday": true, "black friday": true, "holiday special": true, "percent off": true,
		"buy one get one": true, "free gift": true, "limited edition": true, "member exclusive": true,
		"savings": true, "voucher": true, "deal alert": true, "last chance": true,
		"don't miss": true, "rewards": true, "points": true, "cash back": true,
		"view weekly ad": true, "flyer": true, "catalog": true, "lookbook": true,
		"early access": true, "pre-order": true, "special financing": true, "bogo": true,
		"promotional": true, "advertisement": true, "unlock": true, "claim your": true,
		"win a": true, "congratulations you": true, "exclusive access": true, "gift card": true,
		"loyalty program": true, "email only": true, "super sale": true, "spring sale": true,
		"summer sale": true, "fall sale": true, "winter sale": true, "back in stock": true,
		"new collection": true, "click to redeem": true, "earn points": true, "subscriber only": true,
	},
	"social": {
		"friend request": true, "new follower": true, "mention": true, "tagged": true,
		"notification": true, "activity": true, "update": true, "you have a new message": true,
		"connect": true, "join": true, "poke": true, "like": true,
		"comment": true, "share": true, "friend": true, "follower": true,
		"connection": true, "network": true, "profile": true, "feed": true,
		"timeline": true, "post": true, "alert": true, "you were mentioned": true,
		"someone liked your post": true, "new post from": true, "friend activity": true, "social network": true,
		"platform notification": true, "instagram": true, "facebook": true, "twitter": true,
		"linkedin": true, "snapchat": true, "tiktok": true, "pinterest": true,
		"youtube": true, "new connection": true, "message request": true, "shared a photo": true,
		"commented on your photo": true, "added you": true, "started a live video": true, "event invitation": true,
		"rsvp": true, "suggested for you": true, "people you may know": true, "trending": true,
		"popular": true, "live stream": true, "group invite": true, "story update": true,
		"reels": true, "direct message": true, "dm": true, "private message": true,
		"your account activity": true, "follower update": true, "mutual connection": true, "channel update": true,
	},
	"spam": {
		"viagra": true, "free money": true, "lottery": true, "$$$": true,
		"click here": true, "urgent": true, "winner": true, "prize": true,
		"earn money": true, "guaranteed": true, "risk-free": true, "call now": true,
		"cash": true, "credit": true, "loan": true, "pharmacy": true,
		"adult": true, "weight loss": true, "refinance": true, "investment": true,
		"claim your prize": true, "act now": true, "limited offer": true, "win a prize": true,
		"congratulations": true, "you have been selected": true, "free trial": true,
		"nigerian prince": true, "get rich quick": true, "free download": true, "work from home": true,
		"make money": true, "increase your income": true, "double your money": true, "get out of debt": true,
		"unsecured loan": true, "best price": true, "lowest price": true, "satisfaction guaranteed": true,
		"buy direct": true, "cheap": true, "no cost": true, "amazing deal": true,
		"cialis": true, "levitra": true, "miracle cure": true, "lose weight fast": true,
		"erectile dysfunction": true, "enlargement": true, "debt consolidation": true, "advance fee": true,
		"nigeria": true, "your account has been compromised": true, "verify your account": true,
		"password reset required": true, "account suspended": true, "male enhancement": true, "online pharmacy": true,
		"no prescription": true, "hidden fees": true, "confidential": true, "inheritance": true,
		"unclaimed funds": true, "phishing": true, "malware": true, "urgent response required": true,
		"action required": true, "multi-level marketing": true, "mlm": true, "pyramid scheme": true,
		"be your own boss": true, "financial freedom": true, "congratulations!": true, "you've won!": true,
		"pills": true, "sex": true, "adult content": true, "hot singles": true,
		"x-rated": true, "casino": true, "gambling": true, "cryptocurrency investment": true,
		"bitcoin investment": true, "secret method": true, "shocking truth": true, "don't tell anyone": true,
		"no obligation": true, "your bank account": true, "security notification": true, // Often a phishing attempt
		"update your information": true, "suspect activity": true, "paypal limited": true, "amazon locked": true,
		"netflix suspended": true, "your card has been declined": true, "charity donation": true,
		"emergency fund": true, "wire transfer": true, "unsolicited": true, "spam": true, // meta keyword
		"remove me": true, // often in spam emails, though also legitimate newsletters
	},
	"updates": {
		"order confirmation": true, "shipping notification": true, "receipt": true, "bill": true,
		"payment due": true, "account updated": true, "your order": true, "delivery": true,
		"tracking number": true, "invoice": true, "statement": true, "renewal": true,
		"reservation": true, "appointment": true, "alert": true, "update": true,
		"confirmation": true, "booking": true, "account activity": true, "security alert": true,
		"password reset": true, "membership renewal": true, "thank you for your order": true,
		"your package has shipped": true, "delivery confirmation": true, "your bill is ready": true,
		"payment received": true, "upcoming appointment": true, "service update": true,
		"software update": true, "system notification": true, "important information": true,
		"account statement": true, "billing information": true, "order status": true,
		"e-ticket": true, "itinerary": true, "flight confirmation": true, "hotel booking": true,
		"rental car": true, "your recent activity": true, "password changed": true,
		"new login detected": true, "terms of service": true, "privacy policy update": true,
		"subscription confirmation": true, "automatic payment": true, "e-bill": true, "shipment tracking": true,
		"farewell": true, "cancellation": true, "refund": true, "policy update": true,
		"service disruption": true, "maintenance": true, "downtime": true, "service availability": true,
		"status change": true, "event details": true, "webinar confirmation": true,
		"registration confirmed": true, "ticket details": true, "your balance": true, "overdue": true,
		"scheduled maintenance": true, "system message": true, "data breach": true, "outage": true,
	},
}

// CategoryAnalysisResult holds both the final best category and the counts for all categories.
type CategoryAnalysisResult struct {
	BestCategory string
	Counts       map[string]int
}

// --- Globals for the efficient classifier ---
var (
	// The Aho-Corasick matcher, built once at startup.
	categoryMatcher ahocorasick.Matcher

	// A mapping from a keyword's index in the matcher to its category name.
	keywordToCategory []string

	// The order of categories for tie-breaking and processing.
	categoryPriorityOrder = []string{"spam", "social", "promotions", "updates", "forums"}
)

// init() runs once when the package is loaded, before main().
// It's the perfect place to do one-time setup like building our matcher.
func init() {
	// Flatten all keywords from all categories into a single list.
	// We also build a parallel slice that maps each keyword back to its category.
	var allKeywords [][]byte
	for _, categoryName := range categoryPriorityOrder {
		keywords, ok := EmailCategories[categoryName]
		if !ok {
			continue
		}
		for keyword := range keywords {
			allKeywords = append(allKeywords, []byte(keyword))
			keywordToCategory = append(keywordToCategory, categoryName)
		}
	}

	// Build the efficient matcher from all our keywords.
	categoryMatcher = *ahocorasick.NewMatcher(allKeywords)
}

// ClassifyEmail determines the category of an email based on its content.
// This version uses a highly efficient single-pass Aho-Corasick scanner.
func ClassifyEmail(subject, body string) CategoryAnalysisResult {
	// Combine subject and body and convert to lowercase for case-insensitive matching.
	fullContent := strings.ToLower(subject + " " + body)
	contentBytes := []byte(fullContent)

	// Initialize a map to store the counts for all categories.
	// We pre-populate it to ensure all categories are in the final map, even with 0 count.
	allCategoryCounts := make(map[string]int)
	for cat := range EmailCategories {
		allCategoryCounts[cat] = 0
	}

	// --- Step 1: Find all matches in a single pass ---
	// CORRECTED: The method is Match(), not FindAll(). It returns a simple slice of integers.
	// Each integer is the index of a keyword in the dictionary we built in init().
	matches := categoryMatcher.Match(contentBytes)

	// --- Step 2: Tally the counts for each category ---
	// The loop now iterates over a slice of keyword indexes.
	for _, keywordIndex := range matches {
		// CORRECTED: The item in the slice *is* the index. No .Index field is needed.
		// We use this index to look up the category name from our mapping slice.
		categoryName := keywordToCategory[keywordIndex]
		allCategoryCounts[categoryName]++
	}

	// --- Step 3: Determine the best category based on priority and counts ---
	// (This part of the logic remains unchanged and is correct)
	if allCategoryCounts["spam"] > 0 {
		return CategoryAnalysisResult{
			BestCategory: "spam",
			Counts:       allCategoryCounts,
		}
	}

	maxMentions := -1
	bestCategory := "primary"

	for _, categoryName := range categoryPriorityOrder {
		if categoryName == "spam" {
			continue
		}
		if count := allCategoryCounts[categoryName]; count > maxMentions {
			maxMentions = count
			bestCategory = categoryName
		}
	}

	if maxMentions < 5 {
		bestCategory = "primary"
	}

	return CategoryAnalysisResult{
		BestCategory: bestCategory,
		Counts:       allCategoryCounts,
	}
}
