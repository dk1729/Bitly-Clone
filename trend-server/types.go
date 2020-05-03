/*
	Gumball API in Go (Version 1)
	Basic Version with no Backend Services
*/

package main

type shortner struct {
	ID        int
	url       string
	shortlink string
	count     int
}
