// Copyright (c) 2016, Daniel Martí <mvdan@mvdan.cc>
// See LICENSE for licensing information

package syntax

import (
	"bytes"
	"io"
	"unicode/utf8"
)

// bytes that form or start a token
func regOps(r rune) bool {
	switch r {
	case ';', '"', '\'', '(', ')', '$', '|', '&', '>', '<', '`':
		return true
	}
	return false
}

// tokenize these inside parameter expansions
func paramOps(r rune) bool {
	switch r {
	case '}', '#', ':', '-', '+', '=', '?', '%', '[', ']', '/', '^', ',', '@':
		return true
	}
	return false
}

// tokenize these inside arithmetic expansions
func arithmOps(r rune) bool {
	switch r {
	case '+', '-', '!', '*', '/', '%', '(', ')', '^', '<', '>', ':', '=',
		',', '?', '|', '&', ']':
		return true
	}
	return false
}

func wordBreak(r rune) bool {
	switch r {
	case ' ', '\t', '\n', ';', '&', '>', '<', '|', '(', ')', '\r':
		return true
	}
	return false
}

func (p *parser) rune() rune {
retry:
	if p.npos < len(p.bs) {
		if b := p.bs[p.npos]; b < utf8.RuneSelf {
			if p.npos++; b == '\n' {
				p.f.lines = append(p.f.lines, p.getPos())
			}
			if p.litBs != nil {
				p.litBs = append(p.litBs, b)
			}
			r := rune(b)
			p.r = r
			return r
		}
		if p.npos+utf8.UTFMax >= len(p.bs) {
			// we might need up to 4 bytes to read a full
			// non-ascii rune
			p.fill()
		}
		var w int
		p.r, w = utf8.DecodeRune(p.bs[p.npos:])
		if p.litBs != nil {
			p.litBs = append(p.litBs, p.bs[p.npos:p.npos+w]...)
		}
		p.npos += w
		if p.r == utf8.RuneError && w == 1 {
			p.posErr(p.getPos(), "invalid UTF-8 encoding")
		}
	} else {
		if p.r == utf8.RuneSelf {
		} else if p.fill(); p.bs == nil {
			p.npos++
			p.r = utf8.RuneSelf
		} else {
			goto retry
		}
	}
	return p.r
}

func (p *parser) unrune(r rune) {
	if p.r != utf8.RuneSelf {
		p.npos -= utf8.RuneLen(p.r)
		p.r = r
	}
}

// fill reads more bytes from the input src into readBuf. Any bytes that
// had not yet been used at the end of the buffer are slid into the
// beginning of the buffer.
func (p *parser) fill() {
	left := len(p.bs) - p.npos
	p.offs += p.npos
	copy(p.readBuf[:left], p.readBuf[p.npos:])
	var n int
	var err error
	if p.readErr == nil {
		n, err = p.src.Read(p.readBuf[left:])
		p.readErr = err
	} else {
		n, err = 0, p.readErr
	}
	if n == 0 {
		// don't use p.errPass as we don't want to overwrite p.tok
		if err != nil && err != io.EOF {
			p.err = err
		}
		if left > 0 {
			p.bs = p.readBuf[:left]
		} else {
			p.bs = nil
		}
	} else {
		p.bs = p.readBuf[:left+n]
	}
	p.npos = 0
}

func (p *parser) nextKeepSpaces() {
	r := p.r
	if p.pos = p.getPos(); r > utf8.RuneSelf {
		p.pos -= Pos(utf8.RuneLen(r) - 1)
	}
	switch p.quote {
	case paramExpRepl:
		switch r {
		case '}', '/':
			p.tok = p.paramToken(r)
		case '`', '"', '$':
			p.tok = p.dqToken(r)
		default:
			p.advanceLitOther(r)
		}
	case dblQuotes:
		switch r {
		case '`', '"', '$':
			p.tok = p.dqToken(r)
		default:
			p.advanceLitDquote(r)
		}
	case hdocBody, hdocBodyTabs:
		if r == '`' || r == '$' {
			p.tok = p.dqToken(r)
		} else if p.hdocStop == nil {
			p.tok = illegalTok
		} else {
			p.advanceLitHdoc(r)
		}
	case paramExpExp:
		switch r {
		case '}':
			p.rune()
			p.tok = rightBrace
		case '`', '"', '$':
			p.tok = p.dqToken(r)
		default:
			p.advanceLitOther(r)
		}
	default: // sglQuotes
		if r == '\'' {
			p.rune()
			p.tok = sglQuote
		} else {
			p.advanceLitOther(r)
		}
	}
}

func (p *parser) next() {
	if p.r == utf8.RuneSelf {
		p.tok = _EOF
		return
	}
	p.spaced, p.newLine = false, false
	if p.quote&allKeepSpaces != 0 {
		p.nextKeepSpaces()
		return
	}
	r := p.r
skipSpace:
	for {
		switch r {
		case utf8.RuneSelf:
			p.tok = _EOF
			return
		case ' ', '\t', '\r':
			p.spaced = true
			r = p.rune()
		case '\n':
			if p.quote == arithmExprLet || p.quote == hdocWord {
				p.tok = illegalTok
				return
			}
			p.spaced, p.newLine = true, true
			r = p.rune()
			if len(p.heredocs) > p.buriedHdocs && p.err == nil {
				if p.doHeredocs(); p.tok == _EOF {
					return
				}
				r = p.r
			}
		case '\\':
			if !p.peekByte('\n') {
				break skipSpace
			}
			p.rune()
			r = p.rune()
		default:
			break skipSpace
		}
	}
	if p.pos = p.getPos(); r > utf8.RuneSelf {
		p.pos -= Pos(utf8.RuneLen(r) - 1)
	}
	switch {
	case p.quote&allRegTokens != 0:
		switch r {
		case ';', '"', '\'', '(', ')', '$', '|', '&', '>', '<', '`':
			p.tok = p.regToken(r)
		case '#':
			r = p.rune()
			p.newLit(r)
			for r != utf8.RuneSelf && r != '\n' {
				r = p.rune()
			}
			if p.mode&ParseComments > 0 {
				p.f.Comments = append(p.f.Comments, &Comment{
					Hash: p.pos,
					Text: p.endLit(),
				})
			} else {
				p.litBs = nil
			}
			p.next()
		case '?', '*', '+', '@', '!':
			if p.peekByte('(') {
				switch r {
				case '?':
					p.tok = globQuest
				case '*':
					p.tok = globStar
				case '+':
					p.tok = globPlus
				case '@':
					p.tok = globAt
				default: // '!'
					p.tok = globExcl
				}
				p.rune()
				p.rune()
			} else {
				p.advanceLitNone(r)
			}
		default:
			p.advanceLitNone(r)
		}
	case p.quote&allArithmExpr != 0 && arithmOps(r):
		p.tok = p.arithmToken(r)
	case p.quote&allParamExp != 0 && paramOps(r):
		p.tok = p.paramToken(r)
	case p.quote == testRegexp:
		if regOps(r) && r != '(' {
			p.tok = p.regToken(r)
		} else {
			p.advanceLitRe(r)
		}
	case regOps(r):
		p.tok = p.regToken(r)
	default:
		p.advanceLitOther(r)
	}
	if p.err != nil && p.tok != _EOF {
		p.tok = _EOF
	}
}

func (p *parser) peekByte(b byte) bool {
	if p.npos == len(p.bs) && p.readErr == nil {
		p.fill()
	}
	return p.npos < len(p.bs) && p.bs[p.npos] == b
}

func (p *parser) regToken(r rune) token {
	switch r {
	case '\'':
		p.rune()
		return sglQuote
	case '"':
		p.rune()
		return dblQuote
	case '`':
		p.rune()
		return bckQuote
	case '&':
		switch p.rune() {
		case '&':
			p.rune()
			return andAnd
		case '>':
			if !p.bash() {
				break
			}
			if p.rune() == '>' {
				p.rune()
				return appAll
			}
			return rdrAll
		}
		return and
	case '|':
		switch p.rune() {
		case '|':
			p.rune()
			return orOr
		case '&':
			if !p.bash() {
				break
			}
			p.rune()
			return pipeAll
		}
		return or
	case '$':
		switch p.rune() {
		case '\'':
			if !p.bash() {
				break
			}
			p.rune()
			return dollSglQuote
		case '"':
			if !p.bash() {
				break
			}
			p.rune()
			return dollDblQuote
		case '{':
			p.rune()
			return dollBrace
		case '[':
			if !p.bash() {
				break
			}
			p.rune()
			return dollBrack
		case '(':
			if p.rune() == '(' {
				p.rune()
				return dollDblParen
			}
			return dollParen
		}
		return dollar
	case '(':
		if p.rune() == '(' && p.bash() {
			p.rune()
			return dblLeftParen
		}
		return leftParen
	case ')':
		p.rune()
		return rightParen
	case ';':
		switch p.rune() {
		case ';':
			if p.rune() == '&' && p.bash() {
				p.rune()
				return dblSemiFall
			}
			return dblSemicolon
		case '&':
			if !p.bash() {
				break
			}
			p.rune()
			return semiFall
		}
		return semicolon
	case '<':
		switch p.rune() {
		case '<':
			if r = p.rune(); r == '-' {
				p.rune()
				return dashHdoc
			} else if r == '<' && p.bash() {
				p.rune()
				return wordHdoc
			}
			return hdoc
		case '>':
			p.rune()
			return rdrInOut
		case '&':
			p.rune()
			return dplIn
		case '(':
			if !p.bash() {
				break
			}
			p.rune()
			return cmdIn
		}
		return rdrIn
	default: // '>'
		switch p.rune() {
		case '>':
			p.rune()
			return appOut
		case '&':
			p.rune()
			return dplOut
		case '|':
			p.rune()
			return clbOut
		case '(':
			if !p.bash() {
				break
			}
			p.rune()
			return cmdOut
		}
		return rdrOut
	}
}

func (p *parser) dqToken(r rune) token {
	switch r {
	case '"':
		p.rune()
		return dblQuote
	case '`':
		p.rune()
		return bckQuote
	default: // '$'
		switch p.rune() {
		case '{':
			p.rune()
			return dollBrace
		case '[':
			if !p.bash() {
				break
			}
			p.rune()
			return dollBrack
		case '(':
			if p.rune() == '(' {
				p.rune()
				return dollDblParen
			}
			return dollParen
		}
		return dollar
	}
}

func (p *parser) paramToken(r rune) token {
	switch r {
	case '}':
		p.rune()
		return rightBrace
	case ':':
		switch p.rune() {
		case '+':
			p.rune()
			return colPlus
		case '-':
			p.rune()
			return colMinus
		case '?':
			p.rune()
			return colQuest
		case '=':
			p.rune()
			return colAssgn
		}
		return colon
	case '+':
		p.rune()
		return plus
	case '-':
		p.rune()
		return minus
	case '?':
		p.rune()
		return quest
	case '=':
		p.rune()
		return assgn
	case '%':
		if p.rune() == '%' {
			p.rune()
			return dblPerc
		}
		return perc
	case '#':
		if p.rune() == '#' {
			p.rune()
			return dblHash
		}
		return hash
	case '[':
		p.rune()
		return leftBrack
	case '/':
		if p.rune() == '/' {
			p.rune()
			return dblSlash
		}
		return slash
	case '^':
		if p.rune() == '^' {
			p.rune()
			return dblCaret
		}
		return caret
	case ',':
		if p.rune() == ',' {
			p.rune()
			return dblComma
		}
		return comma
	default: // '@'
		p.rune()
		return at
	}
}

func (p *parser) arithmToken(r rune) token {
	switch r {
	case '!':
		if p.rune() == '=' {
			p.rune()
			return nequal
		}
		return exclMark
	case '=':
		if p.rune() == '=' {
			p.rune()
			return equal
		}
		return assgn
	case '(':
		p.rune()
		return leftParen
	case ')':
		p.rune()
		return rightParen
	case '&':
		switch p.rune() {
		case '&':
			p.rune()
			return andAnd
		case '=':
			p.rune()
			return andAssgn
		}
		return and
	case '|':
		switch p.rune() {
		case '|':
			p.rune()
			return orOr
		case '=':
			p.rune()
			return orAssgn
		}
		return or
	case '<':
		switch p.rune() {
		case '<':
			if p.rune() == '=' {
				p.rune()
				return shlAssgn
			}
			return hdoc
		case '=':
			p.rune()
			return lequal
		}
		return rdrIn
	case '>':
		switch p.rune() {
		case '>':
			if p.rune() == '=' {
				p.rune()
				return shrAssgn
			}
			return appOut
		case '=':
			p.rune()
			return gequal
		}
		return rdrOut
	case '+':
		switch p.rune() {
		case '+':
			p.rune()
			return addAdd
		case '=':
			p.rune()
			return addAssgn
		}
		return plus
	case '-':
		switch p.rune() {
		case '-':
			p.rune()
			return subSub
		case '=':
			p.rune()
			return subAssgn
		}
		return minus
	case '%':
		if p.rune() == '=' {
			p.rune()
			return remAssgn
		}
		return perc
	case '*':
		switch p.rune() {
		case '*':
			p.rune()
			return power
		case '=':
			p.rune()
			return mulAssgn
		}
		return star
	case '/':
		if p.rune() == '=' {
			p.rune()
			return quoAssgn
		}
		return slash
	case '^':
		if p.rune() == '=' {
			p.rune()
			return xorAssgn
		}
		return caret
	case ']':
		p.rune()
		return rightBrack
	case ',':
		p.rune()
		return comma
	case '?':
		p.rune()
		return quest
	default: // ':'
		p.rune()
		return colon
	}
}

func (p *parser) newLit(r rune) {
	// don't let r == utf8.RuneSelf go to the second case as RuneLen
	// would return -1
	if r <= utf8.RuneSelf {
		p.litBs = p.litBuf[:1]
		p.litBs[0] = byte(r)
	} else if p.npos <= len(p.bs) {
		w := utf8.RuneLen(r)
		p.litBs = append(p.litBuf[:0], p.bs[p.npos-w:p.npos]...)
	}
}

func (p *parser) discardLit(n int) { p.litBs = p.litBs[:len(p.litBs)-n] }

func (p *parser) endLit() (s string) {
	if p.r == utf8.RuneSelf {
		s = string(p.litBs)
	} else if len(p.litBs) > 0 {
		s = string(p.litBs[:len(p.litBs)-1])
	}
	p.litBs = nil
	return
}

func (p *parser) advanceLitOther(r rune) {
	tok := _LitWord
loop:
	for p.newLit(r); ; r = p.rune() {
		switch r {
		case utf8.RuneSelf:
			break loop
		case '\\': // escaped byte follows
			if r = p.rune(); r == utf8.RuneSelf {
				break loop
			}
			if r == '\n' {
				p.discardLit(2)
			}
		case '\n':
			switch p.quote {
			case sglQuotes, paramExpRepl, paramExpExp:
			default:
				break loop
			}
		case '\'':
			switch p.quote {
			case paramExpExp, paramExpRepl:
			default:
				break loop
			}
		case '"', '`', '$':
			if p.quote != sglQuotes {
				tok = _Lit
				break loop
			}
		case '}':
			if p.quote&allParamExp != 0 {
				break loop
			}
		case '/':
			if p.quote&allParamExp != 0 && p.quote != paramExpExp {
				break loop
			}
		case ']':
			if p.quote&allRbrack != 0 {
				break loop
			}
		case '!', '*':
			if p.quote&allArithmExpr != 0 {
				break loop
			}
		case ':', '=', '%', '?', '^', ',':
			if p.quote&allArithmExpr != 0 || p.quote&allParamReg != 0 {
				break loop
			}
		case '#', '[', '@':
			if p.quote&allParamReg != 0 {
				break loop
			}
		case '+', '-':
			switch p.quote {
			case paramExpInd, paramExpLen, paramExpOff,
				paramExpExp, paramExpRepl, sglQuotes:
			default:
				break loop
			}
		case ' ', '\t', ';', '&', '>', '<', '|', '(', ')', '\r':
			switch p.quote {
			case paramExpExp, paramExpRepl, sglQuotes:
			default:
				break loop
			}
		}
	}
	p.tok, p.val = tok, p.endLit()
}

func (p *parser) advanceLitNone(r rune) {
	p.asPos = 0
	tok := _LitWord
loop:
	for p.newLit(r); ; r = p.rune() {
		switch r {
		case utf8.RuneSelf, ' ', '\t', '\n', '\r', '&', '|', ';', '(', ')':
			break loop
		case '\\': // escaped byte follows
			if r = p.rune(); r == utf8.RuneSelf {
				break loop
			}
			if r == '\n' {
				p.discardLit(2)
			}
		case '>', '<':
			if p.peekByte('(') {
				tok = _Lit
			} else {
				tok = _LitRedir
			}
			break loop
		case '`':
			if p.quote != subCmdBckquo {
				tok = _Lit
			}
			break loop
		case '"', '\'', '$':
			tok = _Lit
			break loop
		case '?', '*', '+', '@', '!':
			if p.peekByte('(') {
				tok = _Lit
				break loop
			}
		case '=':
			p.asPos = len(p.litBs) - 1
			if p.bash() && p.asPos > 0 && p.litBs[len(p.litBs)-2] == '+' {
				p.asPos-- // a+=r
			}
		}
	}
	p.tok, p.val = tok, p.endLit()
}

func (p *parser) advanceLitDquote(r rune) {
	tok := _LitWord
loop:
	for p.newLit(r); ; r = p.rune() {
		switch r {
		case utf8.RuneSelf, '"':
			break loop
		case '\\': // escaped byte follows
			if r = p.rune(); r == utf8.RuneSelf {
				break loop
			}
		case '`', '$':
			tok = _Lit
			break loop
		}
	}
	p.tok, p.val = tok, p.endLit()
}

func (p *parser) advanceLitHdoc(r rune) {
	p.newLit(r)
	if p.quote == hdocBodyTabs {
		for r == '\t' {
			r = p.rune()
		}
	}
	endOff := len(p.litBs) - 1
loop:
	for ; ; r = p.rune() {
		switch r {
		case utf8.RuneSelf, '`', '$':
			break loop
		case '\\': // escaped byte follows
			if r = p.rune(); r == utf8.RuneSelf {
				break loop
			}
		case '\n':
			if bytes.Equal(p.litBs[endOff:len(p.litBs)-1], p.hdocStop) {
				p.discardLit(len(p.hdocStop))
				p.hdocStop = nil
				break loop
			}
			r = p.rune()
			if p.quote == hdocBodyTabs {
				for r == '\t' {
					r = p.rune()
				}
			}
			endOff = len(p.litBs) - 1
		}
	}
	if bytes.Equal(p.litBs[endOff:], p.hdocStop) {
		p.discardLit(len(p.hdocStop))
		p.hdocStop = nil
	}
	p.tok, p.val = _Lit, p.endLit()
}

func (p *parser) hdocLitWord() *Word {
	r := p.r
	p.newLit(r)
	pos := p.getPos()
	for r != utf8.RuneSelf {
		if p.quote == hdocBodyTabs {
			for r == '\t' {
				r = p.rune()
			}
		}
		endOff := len(p.litBs) - 1
		for r != utf8.RuneSelf && r != '\n' {
			r = p.rune()
		}
		if r == utf8.RuneSelf && bytes.Equal(p.litBs[endOff:], p.hdocStop) {
			p.discardLit(len(p.hdocStop))
			break
		}
		if bytes.Equal(p.litBs[endOff:len(p.litBs)-1], p.hdocStop) {
			p.discardLit(len(p.hdocStop))
			break
		}
		r = p.rune()
	}
	l := p.lit(pos, p.endLit())
	return p.word(p.singleWps(l))
}

func (p *parser) advanceLitRe(r rune) {
	lparens := 0
loop:
	for p.newLit(r); ; r = p.rune() {
		switch r {
		case utf8.RuneSelf:
			break loop
		case '(':
			lparens++
		case ')':
			lparens--
		case ' ', '\t', '\r', '\n':
			if lparens == 0 {
				break loop
			}
		}
	}
	p.tok, p.val = _LitWord, p.endLit()
}

func testUnaryOp(val string) token {
	switch val {
	case "!":
		return exclMark
	case "-e", "-a":
		return tsExists
	case "-f":
		return tsRegFile
	case "-d":
		return tsDirect
	case "-c":
		return tsCharSp
	case "-b":
		return tsBlckSp
	case "-p":
		return tsNmPipe
	case "-S":
		return tsSocket
	case "-L", "-h":
		return tsSmbLink
	case "-g":
		return tsGIDSet
	case "-u":
		return tsUIDSet
	case "-r":
		return tsRead
	case "-w":
		return tsWrite
	case "-x":
		return tsExec
	case "-s":
		return tsNoEmpty
	case "-t":
		return tsFdTerm
	case "-z":
		return tsEmpStr
	case "-n":
		return tsNempStr
	case "-o":
		return tsOptSet
	case "-v":
		return tsVarSet
	case "-R":
		return tsRefVar
	default:
		return illegalTok
	}
}

func testBinaryOp(val string) token {
	switch val {
	case "=":
		return assgn
	case "==":
		return equal
	case "!=":
		return nequal
	case "=~":
		return tsReMatch
	case "-nt":
		return tsNewer
	case "-ot":
		return tsOlder
	case "-ef":
		return tsDevIno
	case "-eq":
		return tsEql
	case "-ne":
		return tsNeq
	case "-le":
		return tsLeq
	case "-ge":
		return tsGeq
	case "-lt":
		return tsLss
	case "-gt":
		return tsGtr
	default:
		return illegalTok
	}
}
