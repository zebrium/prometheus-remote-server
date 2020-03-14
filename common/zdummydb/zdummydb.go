// Copyright 2019 Zebrium Inc
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package zdummydb

// This module implements dummy in-memory authenticate module to authenticate
// a given token.
// Ideally, one should be looking up the tokens either from the database or from
// the local files preferably with caching information in in-memory.

// Cred - credentials needed to connect to database/schema
type Cred struct {
	Account   string // Account.
	Token     string // Token.
	SpollSecs int    // stats polling seconds (0 means, no minimum enforced).
}

var creds []Cred = []Cred{
	{Account: "Zebrium", Token: "12345", SpollSecs: 0},
}

// This is a dummy token validation. Ideally, we should be checking from the
// accounts that we have added to database.
func GetTokenCred(token string) (*Cred, error) {
	for i := 0; i < len(creds); i++ {
		if creds[i].Token == token {
			return &creds[i], nil
		}
	}

	// TODO: For now, consider all tokens are valid.
	return &creds[0], nil
}
