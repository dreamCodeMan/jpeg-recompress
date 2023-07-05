package main

import (
	"bytes"
	"errors"
	"image"
	"image/color"
	"image/jpeg"
	"io"
	"log"
	"math"
	"net/http"
	"os"
)

// 默认SSIM常量
var (
	L  = 255.0
	K1 = 0.01
	K2 = 0.03
	C1 = math.Pow((K1 * L), 2.0)
	C2 = math.Pow((K2 * L), 2.0)
)

// 读取图片
func readImage(fname string) (image.Image, error) {
	file, err := os.Open(fname)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	img, _, err := image.Decode(file)
	if err != nil {
		return nil, err
	}

	return img, nil
}

// 判断是否是JPEG格式图像
func isJpeg(path string) bool {
	f, err := os.Open(path)
	if err != nil {
		return false
	}
	defer f.Close()

	buffer := make([]byte, 512)
	_, err = f.Read(buffer)
	if err != nil {
		return false
	}
	contentType := http.DetectContentType(buffer)
	return contentType == "image/jpeg"
}

// 获得文件大小
func getFilesize(path string) (size int64, err error) {
	fi, err := os.Stat(path)
	if err != nil {
		return
	}
	size = fi.Size()
	return
}

// 返回指定质量的图片的byte值
func encodeToJPEGBytes(img image.Image, quality int) ([]byte, error) {
	options := &jpeg.Options{
		Quality: quality,
	}
	buf := new(bytes.Buffer)
	err := jpeg.Encode(buf, img, options)
	if err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

// 转换为灰阶
func convertToGray(originalImg image.Image) image.Image {
	bounds := originalImg.Bounds()
	w, h := dim(originalImg)

	grayImg := image.NewGray(bounds)

	for x := 0; x < w; x++ {
		for y := 0; y < h; y++ {
			originalColor := originalImg.At(x, y)
			grayColor := color.GrayModel.Convert(originalColor)
			grayImg.Set(x, y, grayColor)
		}
	}

	return grayImg
}

// 将uint32类型的R值转换为float64类型。返回的float值将在0-255的范围内。
func getPixVal(c color.Color) float64 {
	r, _, _, _ := c.RGBA()
	return float64(r >> 8)
}

// 返回图像的宽和高
func dim(img image.Image) (w, h int) {
	w, h = img.Bounds().Max.X, img.Bounds().Max.Y
	return
}

// 检查两个图像是否具有相同的尺寸
func equalDim(img1, img2 image.Image) bool {
	w1, h1 := dim(img1)
	w2, h2 := dim(img2)
	return (w1 == w2) && (h1 == h2)
}

// 给定一个图像，计算其像素值的平均值
func mean(img image.Image) float64 {
	w, h := dim(img)
	n := float64((w * h) - 1)
	sum := 0.0

	for x := 0; x < w; x++ {
		for y := 0; y < h; y++ {
			sum += getPixVal(img.At(x, y))
		}
	}
	return sum / n
}

// 使用图像的像素值计算标准差
func stdev(img image.Image) float64 {
	w, h := dim(img)

	n := float64((w * h) - 1)
	sum := 0.0
	avg := mean(img)

	for x := 0; x < w; x++ {
		for y := 0; y < h; y++ {
			pix := getPixVal(img.At(x, y))
			sum += math.Pow((pix - avg), 2.0)
		}
	}
	return math.Sqrt(sum / n)
}

// 计算图像的方差
func covar(img1, img2 image.Image) (c float64, err error) {
	if !equalDim(img1, img2) {
		err = errors.New("images must have same dimension")
		return
	}
	avg1 := mean(img1)
	avg2 := mean(img2)
	w, h := dim(img1)
	sum := 0.0
	n := float64((w * h) - 1)

	for x := 0; x < w; x++ {
		for y := 0; y < h; y++ {
			pix1 := getPixVal(img1.At(x, y))
			pix2 := getPixVal(img2.At(x, y))
			sum += (pix1 - avg1) * (pix2 - avg2)
		}
	}
	c = sum / n
	return
}

// 计算两个图像的结构相似性SSIM
func ssim(x, y image.Image) float64 {
	avgX := mean(x)
	avgY := mean(y)

	stdevX := stdev(x)
	stdevY := stdev(y)

	cov, err := covar(x, y)
	if err != nil {
		return 0.0
	}

	numerator := ((2.0 * avgX * avgY) + C1) * ((2.0 * cov) + C2)
	denominator := (math.Pow(avgX, 2.0) + math.Pow(avgY, 2.0) + C1) * (math.Pow(stdevX, 2.0) + math.Pow(stdevY, 2.0) + C2)

	return numerator / denominator
}

// 返回压缩后托的SSIM和图片大小
func compare(original image.Image, quality int) (index float64, raw []byte, err error) {
	raw, err = encodeToJPEGBytes(original, quality)
	if err != nil {
		return
	}
	decoded, err := jpeg.Decode(bytes.NewReader(raw))
	if err != nil {
		return
	}
	index = ssim(original, convertToGray(decoded))
	return
}

// 写入文件
func save(p string, data []byte) (err error) {
	f, err := os.Create(p)
	if err != nil {
		log.Println(err)
	}
	defer f.Close()
	f.Write(data)
	return
}

// 复制文件
func copyFile(src string, dest string) (nBytes int64, err error) {
	source, err := os.Open(src)
	if err != nil {
		return 0, err
	}
	defer source.Close()

	destination, err := os.Create(dest)
	if err != nil {
		return 0, err
	}
	defer destination.Close()
	nBytes, err = io.Copy(destination, source)
	return nBytes, err
}
