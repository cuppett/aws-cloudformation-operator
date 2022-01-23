/*
MIT License

Copyright (c) 2022 Stephen Cuppett

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in all
copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
SOFTWARE.
*/

package servicesk8saws

import (
	"context"
	"github.com/aws/aws-sdk-go-v2/aws"
	v1 "k8s.io/api/core/v1"
)

type SecretProvider struct {
	v1.Secret
}

func (p *SecretProvider) Retrieve(ctx context.Context) (creds aws.Credentials, err error) {
	value, exists := p.Secret.Data["aws_access_key_id"]
	if exists {
		creds.AccessKeyID = string(value)
	}
	value, exists = p.Secret.Data["aws_secret_access_key"]
	if exists {
		creds.SecretAccessKey = string(value)
	}

	return creds, nil
}
