/* Copyright © 2025 Mike Brown. All Rights Reserved.
 *
 * See LICENSE file at the root of this repository for license terms
 */
package bcc

import (
	"testing"
)

func TestActivePlayerMemIds(t *testing.T) {
	ids := ActivePlayerMemIds()
	if len(ids) < 400 {
		t.Fatalf("ActivePlayerMemIds() too small")
	}
}

func TestActivePlayerTIds(t *testing.T) {
	ids := ActivePlayerTIds()
	if len(ids) < 400 {
		t.Fatalf("ActivePlayerTIds() too small")
	}
}
