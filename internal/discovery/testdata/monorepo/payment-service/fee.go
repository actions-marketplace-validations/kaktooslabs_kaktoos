package payment

// Fee returns the fee for an amount.
func Fee(amount int) int { return amount / 100 }
