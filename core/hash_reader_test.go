package core

import (
	"crypto/md5"
	"io/ioutil"
	"strings"
	"testing"

	"github.com/smartystreets/assertions/should"
	"github.com/smartystreets/gunit"
)

func TestHashReaderFixture(t *testing.T) {
	gunit.Run(new(HashReaderFixture), t)
}

type HashReaderFixture struct {
	*gunit.Fixture
}

func (this *HashReaderFixture) Test() {
	stuff := strings.Repeat("Hello, World!", 1024)
	expected := md5.New()
	expected.Write([]byte(stuff))
	data := strings.NewReader(stuff)
	hasher := md5.New()

	_, _ = ioutil.ReadAll(NewHashReader(data, hasher))

	this.So(hasher.Sum(nil), should.Resemble, expected.Sum(nil))
}
