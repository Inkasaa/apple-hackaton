package main

import "sort"

// sortSlice sorts a slice of AdminCustomer in place using the provided less function
func sortSlice(slice []AdminCustomer, less func(a, b AdminCustomer) bool) {
	sort.Slice(slice, func(i, j int) bool {
		return less(slice[i], slice[j])
	})
}
