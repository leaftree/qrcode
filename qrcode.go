package qrcode

import (
    "math"
    "git.spiritframe.com/tuotoo/utils"
    "fmt"
    "image"
    "strconv"
    "os"
    "image/png"
    "image/color"
)

type PositionDetectionPatterns struct {
	Topleft *PosGroup
	Right   *PosGroup
	Bottom  *PosGroup
}

type PosGroup struct {
	Group    []Pos
	GroupMap map[Pos]bool
	Min      Pos
	Max      Pos
	Center   Pos
	Kong     bool
}

type Matrix struct {
	Points [][]bool
}

func (m *Matrix) At(x, y int) bool {
	t := 0
	f := 0
	for i := -1; i < 2; i++ {
		for j := -1; j < 2; j++ {
			if m.Points[y+i][x+j] {
				t += 1
			} else {
				f += 1
			}
		}
	}
	if t > f {
		return true
	}
	return false
}

func (m *Matrix) FormatInfo() (ErrorCorrectionLevel, Mask int) {
	fi1 := []Pos{
		{0, 8}, {1, 8}, {2, 8}, {3, 8}, {4, 8}, {5, 8}, {7, 8}, {8, 8},
		{8, 7}, {8, 5}, {8, 4}, {8, 3}, {8, 2}, {8, 1}, {8, 0},
	}
	maskedfidata := m.GetBin(fi1)
	unmaskfidata := maskedfidata ^ 0x5412
	if bch(unmaskfidata) == 0 {
		ErrorCorrectionLevel = unmaskfidata >> 13
		Mask = unmaskfidata >> 10 & 7
		return
	}
	length := len(m.Points)
	fi2 := []Pos{
		{8, length - 1}, {8, length - 2}, {8, length - 3}, {8, length - 4}, {8, length - 5}, {8, length - 6}, {8, length - 7},
		{length - 8, 8}, {length - 7, 8}, {length - 6, 8}, {length - 5, 8}, {length - 4, 8}, {length - 3, 8}, {length - 2, 8}, {length - 1, 8},
	}
	maskedfidata = m.GetBin(fi2)
	unmaskfidata = maskedfidata ^ 0x5412
	if bch(unmaskfidata) == 0 {
		ErrorCorrectionLevel = unmaskfidata >> 13
		Mask = unmaskfidata >> 10 & 7
		return
	}
	panic("not found errorcorrectionlevel and mask")
}

func (m *Matrix) GetBin(poss []Pos) int {
	var fidata int
	for _, pos := range poss {
		if m.Points[pos.Y][pos.X] {
			fidata = fidata<<1 + 1
		} else {
			fidata = fidata << 1
		}
	}
	return fidata
}

func (m *Matrix) Version() int {
	width := len(m.Points)
	return (width-21)/4 + 1
}

type Pos struct {
	X int
	Y int
}

func bch(org int) int {
	var g int = 0x537
	for i := 4; i > -1; i-- {
		if org&(1<<(uint(i+10))) > 0 {
			org ^= g << uint(i)
		}
	}
	return org
}


func (m *Matrix) DataArea() *Matrix {
    da := new(Matrix)
    width := len(m.Points)
    maxpos := width - 1
    for _, line := range m.Points {
        l := []bool{}
        for range line {
            l = append(l, true)
        }
        da.Points = append(da.Points, l)
    }
    // Position Detection Pattern是定位图案，用于标记二维码的矩形大小。
    // 这三个定位图案有白边叫Separators for Postion Detection Patterns。之所以三个而不是四个意思就是三个就可以标识一个矩形了。
    for y := 0; y < 9; y++ {
        for x := 0; x < 9; x++ {
            da.Points[y][x] = false //左上
        }
    }
    for y := 0; y < 9; y++ {
        for x := 0; x < 8; x++ {
            da.Points[y][maxpos-x] = false //右上
        }
    }
    for y := 0; y < 8; y++ {
        for x := 0; x < 9; x++ {
            da.Points[maxpos-y][x] = false //左下
        }
    }
    // Timing Patterns也是用于定位的。原因是二维码有40种尺寸，尺寸过大了后需要有根标准线，不然扫描的时候可能会扫歪了。
    for i := 0; i < width; i++ {
        da.Points[6][i] = false
        da.Points[i][6] = false
    }
    //Alignment Patterns 只有Version 2以上（包括Version2）的二维码需要这个东东，同样是为了定位用的。
    version := da.Version()
    Alignments := AlignmentPatternCenter[version]
    for _, AlignmentX := range Alignments {
        for _, AlignmentY := range Alignments {
            if (AlignmentX == 6 && AlignmentY == 6) || (maxpos-AlignmentX == 6 && AlignmentY == 6) || (AlignmentX == 6 && maxpos-AlignmentY == 6) {
                continue
            }
            for y := AlignmentY - 2; y <= AlignmentY+2; y++ {
                for x := AlignmentX - 2; x <= AlignmentX+2; x++ {
                    da.Points[y][x] = false
                }
            }
        }
    }
    //Version Information 在 >= Version 7以上，需要预留两块3 x 6的区域存放一些版本信息。
    if version >= 7 {
        for i := maxpos - 8; i < maxpos-11; i++ {
            for j := 0; j < 6; j++ {
                da.Points[i][j] = false
                da.Points[j][i] = false
            }
        }
    }
    return da
}

func NewPositionDetectionPattern(pdps [][][]Pos) *PositionDetectionPatterns {
    if len(pdps) < 3 {
        panic("缺少pdp")
    }
    pdpgroups := []*PosGroup{}
    for _, pdp := range pdps {
        pdpgroups = append(pdpgroups, PosslistToGroup(pdp))
    }
    ks := []*K{}
    for i, firstpdpgroup := range pdpgroups {
        for j, lastpdpgroup := range pdpgroups {
            if i == j {
                continue
            }
            k := &K{FirstPosGroup: firstpdpgroup, LastPosGroup: lastpdpgroup}
            Radian(k)
            ks = append(ks, k)
        }
    }
    var Offset float64 = 360
    var KF, KL *K
    for i, kf := range ks {
        for j, kl := range ks {
            if i == j {
                continue
            }
            if kf.FirstPosGroup != kl.FirstPosGroup {
                continue
            }
            offset := IsVertical(kf, kl)
            if offset < Offset {
                Offset = offset
                KF = kf
                KL = kl
            }
        }
    }
    positionDetectionPatterns := new(PositionDetectionPatterns)
    positionDetectionPatterns.Topleft = KF.FirstPosGroup
    positionDetectionPatterns.Bottom = KL.LastPosGroup
    positionDetectionPatterns.Right = KF.LastPosGroup
    return positionDetectionPatterns
}


func PosslistToGroup(groups [][]Pos) *PosGroup {
    newgroup := []Pos{}
    for _, group := range groups {
        newgroup = append(newgroup, group...)
    }
    return PossToGroup(newgroup)
}

type K struct {
    FirstPosGroup *PosGroup
    LastPosGroup  *PosGroup
    K             float64
}

func Radian(k *K) {
    x, y := k.LastPosGroup.Center.X-k.FirstPosGroup.Center.X, k.LastPosGroup.Center.Y-k.FirstPosGroup.Center.Y
    k.K = math.Atan2(float64(y), float64(x))
}

func IsVertical(kf, kl *K) (offset float64) {
    dk := kl.K - kf.K
    offset = math.Abs(dk - math.Pi/2)
    return
}


func PossToGroup(group []Pos) *PosGroup {
    posgroup := new(PosGroup)
    posgroup.Group = group
    posgroup.Center = CenterPoint(group)
    var mapgroup = map[Pos]bool{}
    for _, pos := range group {
        mapgroup[pos] = true
    }
    posgroup.GroupMap = mapgroup
    minx, maxx, miny, maxy := Rectangle(group)
    posgroup.Kong = Kong(mapgroup, minx, maxx, miny, maxy)
    posgroup.Min = Pos{X: minx, Y: miny}
    posgroup.Max = Pos{X: maxx, Y: maxy}
    return posgroup
}

func check(err error) bool {
    return utils.Check(err)
}


func Rectangle(group []Pos) (minx, maxx, miny, maxy int) {
    minx, maxx, miny, maxy = group[0].X, group[0].X, group[0].Y, group[0].Y

    for _, pos := range group {
        if pos.X < minx {
            minx = pos.X
        }
        if pos.X > maxx {
            maxx = pos.X
        }
        if pos.Y < miny {
            miny = pos.Y
        }
        if pos.Y > maxy {
            maxy = pos.Y
        }
    }
    return
}


func CenterPoint(group []Pos) Pos {
    sumx, sumy := 0, 0
    for _, pos := range group {
        sumx += pos.X
        sumy += pos.Y
    }
    meanx := sumx / len(group)
    meany := sumy / len(group)
    return Pos{X: meanx, Y: meany}
}

func MaskFunc(code int) func(x, y int) bool {
    switch code {
    case 0: //000
        return func(x, y int) bool {
            return (x+y)%2 == 0
        }
    case 1: //001
        return func(x, y int) bool {
            return y%2 == 0
        }
    case 2: //010
        return func(x, y int) bool {
            return x%3 == 0
        }
    case 3: //011
        return func(x, y int) bool {
            return (x+y)%3 == 0
        }
    case 4: // 100
        return func(x, y int) bool {
            return (y/2+x/3)%2 == 0
        }
    case 5: // 101
        return func(x, y int) bool {
            return (x*y)%2+(x*y)%3 == 0
        }
    case 6: // 110
        return func(x, y int) bool {
            return ((x*y)%2+(x*y)%3)%2 == 0
        }
    case 7: // 111
        return func(x, y int) bool {
            return ((x+y)%2+(x*y)%3)%2 == 0
        }
    }
    return func(x, y int) bool {
        return false
    }
}


func SplitGroup(poss map[Pos]bool, pos Pos) []Pos {
    around := []Pos{}
    for y := -1; y < 2; y++ {
        for x := -1; x < 2; x++ {
            here := Pos{X: pos.X + x, Y: pos.Y + y}
            _, ok := poss[here]
            if ok {
                delete(poss, here)
                around = append(around, here)
            }
        }
    }
    for _, v := range around {
        for _, p := range SplitGroup(poss, v) {
            around = append(around, p)
        }
    }
    return around
}


func Kong(groupmap map[Pos]bool, minx, maxx, miny, maxy int) bool {
    count := 0
    for x := minx; x <= maxx; x++ {
        dian := false
        last := false
        for y := miny; y <= maxy; y++ {
            if _, ok := groupmap[Pos{X: x, Y: y}]; ok {
                if !last {
                    if dian {
                        if count > 2 {
                            return true
                        }
                    } else {
                        dian = true
                    }
                }
                last = true
            } else {
                last = false
                if dian {
                    count++
                }
            }
        }
    }
    return false
}


func ParseBlock(m *Matrix, data []bool) ([]bool, []bool) {
    version := m.Version()
    level, _ := m.FormatInfo()
    var qrcodeversion = QRcodeVersion{}
    for _, qrcodeVersion := range Versions {
        if qrcodeVersion.Level == RecoveryLevel(level) && qrcodeVersion.Version == version {
            qrcodeversion = qrcodeVersion
        }
    }

    dataBlocks := [][]bool{}
    for _, block := range qrcodeversion.Block {
        for i := 0; i < block.NumBlocks; i++ {
            dataBlocks = append(dataBlocks, []bool{})
        }
    }
    for {
        leftlength := len(data)
        no := 0
        for _, block := range qrcodeversion.Block {
            for i := 0; i < block.NumBlocks; i++ {
                if len(dataBlocks[no]) < block.NumDataCodewords*8 {
                    dataBlocks[no] = append(dataBlocks[no], data[0:8]...)
                    data = data[8:]
                }
                no += 1
            }
        }
        if leftlength == len(data) {
            break
        }
    }
    datacode := []bool{}
    for _, block := range dataBlocks {
        datacode = append(datacode, block...)
    }

    errorBlocks := [][]bool{}
    for _, block := range qrcodeversion.Block {
        for i := 0; i < block.NumBlocks; i++ {
            errorBlocks = append(errorBlocks, []bool{})
        }
    }
    for {
        leftlength := len(data)
        no := 0
        for _, block := range qrcodeversion.Block {
            for i := 0; i < block.NumBlocks; i++ {
                if len(errorBlocks[no]) < (block.NumCodewords-block.NumDataCodewords)*8 {
                    errorBlocks[no] = append(errorBlocks[no], data[:8]...)
                    if len(data) > 8 {
                        data = data[8:]
                    }
                }
                no += 1
            }
        }
        if leftlength == len(data) {
            break
        }
    }
    errorcode := []bool{}
    for _, block := range errorBlocks {
        errorcode = append(errorcode, block...)
    }

    return datacode, errorcode
}


func LineWidth(positionDetectionPatterns [][][]Pos) float64 {
    sumwidth := 0
    for _, positionDetectionPattern := range positionDetectionPatterns {
        for _, group := range positionDetectionPattern {
            minx, maxx, miny, maxy := Rectangle(group)
            sumwidth += maxx - minx + 1
            sumwidth += maxy - miny + 1
        }
    }
    return float64(sumwidth) / 60
}

func IsPositionDetectionPattern(bukonggroup, konggroup []Pos) bool {
    buminx, bumaxx, buminy, bumaxy := Rectangle(bukonggroup)
    minx, maxx, miny, maxy := Rectangle(konggroup)
    if !(buminx > minx && bumaxx > minx &&
        buminx < maxx && bumaxx < maxx &&
        buminy > miny && bumaxy > miny &&
        buminy < maxy && bumaxy < maxy) {
        return false
    }
    kongcenter := CenterPoint(konggroup)
    if !(kongcenter.X > buminx && kongcenter.X < bumaxx &&
        kongcenter.Y > buminy && kongcenter.Y < bumaxy) {
        return false
    }
    return true
}


func GetData(unmaskmatrix, dataarea *Matrix) []bool {
    width := len(unmaskmatrix.Points)
    data := []bool{}
    maxpos := width - 1
    for t := maxpos; t > 0; {
        for y := maxpos; y >= 0; y-- {
            for x := t; x >= t-1; x-- {
                if dataarea.Points[y][x] {
                    data = append(data, unmaskmatrix.Points[y][x])
                }
            }
        }
        t = t - 2
        if t == 6 {
            t = t - 1
        }
        for y := 0; y <= maxpos; y++ {
            for x := t; x >= t-1; x-- {
                if dataarea.Points[y][x] {
                    data = append(data, unmaskmatrix.Points[y][x])
                }
            }
        }
        t = t - 2
    }
    return data
}


func Bits2Bytes(datacode []bool, version int) []byte {
    format := Bit2Int(datacode[0:4])
    offset := GetDataEncoder(version).CharCountBits(format)
    length := Bit2Int(datacode[4 : 4+offset])
    datacode = datacode[4+offset : length*8+4+offset]
    result := []byte{}
    for i := 0; i < length*8; {
        result = append(result, Bit2Byte(datacode[i:i+8]))
        i += 8
    }
    return result
}

func StringBool(datacode []bool) string {
    return StringByte(Bool2Byte(datacode))
}

func StringByte(b []byte) string {
    var bitString string
    for i := 0; i < len(b)*8; i++ {
        if (i % 8) == 0 {
            bitString += " "
        }

        if (b[i/8] & (0x80 >> byte(i%8))) != 0 {
            bitString += "1"
        } else {
            bitString += "0"
        }
    }

    return fmt.Sprintf("numBits=%d, bits=%s", len(b)*8, bitString)
}

func Bool2Byte(datacode []bool) []byte {
    result := []byte{}
    for i := 0; i < len(datacode); {
        result = append(result, Bit2Byte(datacode[i:i+8]))
        i += 8
    }
    return result
}
func Bit2Int(bits []bool) int {
    g := 0
    for _, i := range bits {
        g = g << 1
        if i {
            g += 1
        }
    }
    return g
}

func Bit2Byte(bits []bool) byte {
    var g uint8
    for _, i := range bits {
        g = g << 1
        if i {
            g += 1
        }
    }
    return byte(g)
}



func Line(start, end *Pos, matrix *Matrix) (line []bool) {
    if math.Abs(float64(start.X-end.X)) > math.Abs(float64(start.Y-end.Y)) {
        length := (end.X - start.X)
        if length > 0 {
            for i := 0; i <= length; i++ {
                k := float64(end.Y-start.Y) / float64(length)
                x := start.X + i
                y := start.Y + int(k*float64(i))
                //logger.Println(x,y,matrix.Points[y][x])
                line = append(line, matrix.Points[y][x])
            }
        } else {
            for i := 0; i >= length; i-- {
                k := float64(end.Y-start.Y) / float64(length)
                x := start.X + i
                y := start.Y + int(k*float64(i))
                //logger.Println(x,y,matrix.Points[y][x])
                line = append(line, matrix.Points[y][x])
            }
        }
    } else {
        length := (end.Y - start.Y)
        if length > 0 {
            for i := 0; i <= length; i++ {
                k := float64(end.X-start.X) / float64(length)
                y := start.Y + i
                x := start.X + int(k*float64(i))
                //logger.Println(x,y,matrix.Points[y][x])
                line = append(line, matrix.Points[y][x])
            }
        } else {
            for i := 0; i >= length; i-- {
                k := float64(end.X-start.X) / float64(length)
                y := start.Y + i
                x := start.X + int(k*float64(i))
                //logger.Println(x,y,matrix.Points[y][x])
                line = append(line, matrix.Points[y][x])
            }
        }
    }
    return
}

// 标线
func Centerlist(line []bool, offset int) (li []int) {
    submap := map[int]int{}
    value := line[0]
    sublength := 0
    for _, b := range line {
        if b == value {
            sublength += 1
        } else {
            _, ok := submap[sublength]
            if ok {
                submap[sublength] += 1
            } else {
                submap[sublength] = 1
            }
            value = b
            sublength = 1
        }
    }
    maxcountsublength := 0
    var meansublength int
    for k, v := range submap {
        if v > maxcountsublength {
            maxcountsublength = v
            meansublength = k
        }
    }
    start := false
    curvalue := false
    curgroup := []int{}
    for i, v := range line {
        if v == false {
            start = true
        }
        if !start {
            continue
        }
        if v != curvalue {
            if len(curgroup) > meansublength/2 && len(curgroup) < meansublength+meansublength/2 {
                curvalue = v
                mean := 0
                for _, index := range curgroup {
                    mean += index
                }
                li = append(li, mean/len(curgroup)+offset)
                curgroup = []int{}
            } else {
                curgroup = append(curgroup, i)
            }
        } else {
            curgroup = append(curgroup, i)
        }
    }
    if len(curgroup) > meansublength/2 && len(curgroup) < meansublength+meansublength/2 {
        mean := 0
        for _, index := range curgroup {
            mean += index
        }
        li = append(li, mean/len(curgroup)+offset)
    }
    return li
    // todo: jiaodu
}



func ExportEveryGroup(size image.Rectangle, kong [][]Pos, filename string) {
    for i, group := range kong {
        ExportGroup(size, group, filename+strconv.FormatInt(int64(i), 10))
    }
}

func ExportGroups(size image.Rectangle, kong [][]Pos, filename string) {
    result := image.NewGray(size)
    for _, group := range kong {
        for _, pos := range group {
            result.Set(pos.X, pos.Y, color.White)
        }
    }
    firesult, err := os.Create(filename + ".png")
    if !check(err) {
        panic(err)
    }
    defer firesult.Close()
    png.Encode(firesult, result)
}

func ExportGroup(size image.Rectangle, group []Pos, filename string) {
    result := image.NewGray(size)
    for _, pos := range group {
        result.Set(pos.X, pos.Y, color.White)
    }
    firesult, err := os.Create(filename + ".png")
    if !check(err) {
        panic(err)
    }
    defer firesult.Close()
    png.Encode(firesult, result)
}

func ExportMatrix(size image.Rectangle, matrix *Matrix, filename string) {
    result := image.NewGray(size)
    for y, line := range matrix.Points {
        for x, value := range line {
            var c color.Color
            if value {
                c = color.Black
            } else {
                c = color.White
            }
            result.Set(x, y, c)
        }
    }
    firesult, err := os.Create(filename + ".png")
    if !check(err) {
        panic(err)
    }
    defer firesult.Close()
    png.Encode(firesult, result)
}