package coap

//Option Numbers
const (
	OptIfMatch       uint16 = 1
	OptURIHost              = 3
	OptETag                 = 4
	OptIfNoneMatch          = 5
	OptObserve              = 6
	OptURIPort              = 7
	OptLocationPath         = 8
	OptURIPath              = 11
	OptContentFormat        = 12
	OptMaxAge               = 14
	OptURIQuery             = 15
	OptAccept               = 17
	OptLocationQuery        = 20
	OptProxyURI             = 35
	OptProxyScheme          = 39
	OptSize1                = 60
)

type Option struct {
	Number uint16
	Value  []byte
}

func (opt Option) Len() int {
	return len(opt.Value)
}

type Options []Option

func (opts Options) Len() int {
	return len(opts)
}

func (opts Options) Less(i, j int) bool {
	if opts[i].Number == opts[j].Number {
		return i < j
	}

	return opts[i].Number < opts[j].Number
}

func (opts Options) Swap(i, j int) {
	opts[i], opts[j] = opts[j], opts[i]
}

func (msg *Message) GetOptions(number uint16) Options {

	optSelected := make([]Option, 0, len(msg.Options))

	for _, v := range msg.Options {
		if v.Number == number {
			optSelected = append(optSelected, v)
		}

	}

	return optSelected
}

func (msg *Message) UriPath() []string {
	optsPath := msg.GetOptions(OptURIPath)

	PathStrings := make([]string, len(optsPath))

	for i, v := range optsPath {
		PathStrings[i] = string(v.Value[:v.Len()])
	}

	return PathStrings
}

func (msg *Message) UriQuery() []string {
	optsQuery := msg.GetOptions(OptURIQuery)

	QueryStrings := make([]string, len(optsQuery))

	for i, v := range optsQuery {
		QueryStrings[i] = string(v.Value[:v.Len()])
	}

	return QueryStrings
}

const (
	CfTextPlain   uint16 = 0
	CfLinkFormat         = 40
	CfAppXML             = 41
	CfOctetStream        = 42
	CfEXI                = 47
	CfJSON               = 50
)

func (msg *Message) addOption(opt Option, unique bool) {

	if unique {
		msg.removeOptions(opt.Number)
	}

	msg.Options = append(msg.Options, opt)
}

func (msg *Message) removeOptions(number uint16) {

	newOpts := Options{}

	for _, opt := range msg.Options {
		if opt.Number != number {
			newOpts = append(newOpts, opt)
		}
	}

	msg.Options = newOpts
}

func (msg *Message) RemoveOptions(number uint16) {

	msg.removeOptions(number)

}

func (msg *Message) AddContentFormatOpt(cf uint16) {

	if cf == 0 {
		msg.removeOptions(OptContentFormat) //cf=0 is implicit set if option not present
		return
	}

	cfOpt := Option{Number: uint16(OptContentFormat), Value: nil}

	if cf > 0 && cf <= 0xff {
		cfOpt.Value = make([]byte, 1)
		cfOpt.Value[0] = byte(cf)
	} else if cf > 0xff {
		cfOpt.Value = make([]byte, 2)
		cfOpt.Value[0] = byte(cf & 0xff)
		cfOpt.Value[1] = byte(cf >> 8)
	}

	msg.addOption(cfOpt, true)
}

func (msg *Message) AddBlkOption(BlkOptNumber int, BlockNum, Blocksize int, MoreFlag bool) {

	blkOpt := Option{Number: uint16(BlkOptNumber), Value: nil}

	if !(BlkOptNumber == 27 || BlkOptNumber == 23) {
		return
	}

	if BlockNum == 0 && Blocksize == 16 && MoreFlag == false {
		msg.Options = append(msg.Options, blkOpt)
		return
	}

	var OptionVal uint32 = 0

	//Block Size
	var szxCalc uint8 = uint8(Blocksize >> 4) // divide by 16
	for i := 6; i >= 0; i-- {
		if szxCalc&(1<<uint32(i)) > 0 {
			OptionVal |= uint32(i)
		}
	}

	//More Flag
	if MoreFlag {
		OptionVal |= 1 << 3
	}

	OptionVal |= uint32(BlockNum << 4)

	if BlockNum < 16 {
		blkOpt.Value = make([]byte, 1)
		blkOpt.Value[0] = byte(OptionVal)
	} else if BlockNum < 4096 {
		blkOpt.Value = make([]byte, 2)
		blkOpt.Value[0] = byte((OptionVal & 0xffff) >> 8)
		blkOpt.Value[1] = byte((OptionVal & 0xff))
	} else {
		blkOpt.Value = make([]byte, 3)
		blkOpt.Value[0] = byte((OptionVal & 0xffffff) >> 16)
		blkOpt.Value[1] = byte((OptionVal & 0xffff) >> 8)
		blkOpt.Value[2] = byte((OptionVal & 0xff))
	}
	msg.Options = append(msg.Options, blkOpt)
}
