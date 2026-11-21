# Copyright 1999-2026 Gentoo Authors
# Distributed under the terms of the GNU General Public License v2

EAPI=8

inherit go-module

DESCRIPTION="TUI, CLI y API para configurar el teclado AULA/Epomaker F108 Pro"
HOMEPAGE="https://github.com/ldgnu/openaula"

LICENSE="MIT BSD Apache-2.0"
SLOT="0"
KEYWORDS="~amd64 ~arm64 ~x86"
IUSE=""

RDEPEND=""
BDEPEND=">=dev-lang/go-1.24"

SRC_URI="
	https://github.com/ldgnu/openaula/archive/refs/tags/v${PV}.tar.gz -> ${P}.tar.gz
	https://github.com/ldgnu/openaula/releases/download/v${PV}/${P}-vendor.tar.gz
"

S="${WORKDIR}/openaula-${PV}"

src_compile() {
	ego build -trimpath -ldflags "-s -w" -o openaula ./cmd/openaula
}

src_install() {
	dobin openaula
	einstalldocs
	dodoc NOTICE.md
	insinto /usr/share/doc/${PF}
	doins docs/PROTOCOLO.md
	insinto /usr/lib/udev/rules.d
	doins packaging/udev/99-openaula.rules
}
