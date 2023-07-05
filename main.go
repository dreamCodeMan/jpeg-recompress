package main

import (
	"flag"
	"fmt"
	"math"
	"os"
)

// 检查命令行参数
func checkArgs(src string, dest string, force bool, max int, min int, target float64, loops int) bool {
	var msg string
	if _, err := os.Stat(src); os.IsNotExist(err) {
		msg = "Source image '" + src + "' does not exists."
	}
	if !force {
		if _, err := os.Stat(dest); err == nil {
			msg = "Destiation path '" + dest + "' already exists. Use -f to overwrite."
		}
	}
	if dest == "" {
		msg = "Please specify a destination path"
	}
	if max < 1 || max > 100 {
		msg = "Maximum quality has to be between 1 and 100."
	}
	if min < 0 || min > 99 {
		msg = "Minimum quality has to be between 0 and 99."
	}
	if target <= 0 || target > 1 {
		msg = "Target has to be between 0 and 99."
	}
	if loops <= 0 {
		msg = "Loops has to be more than 0"
	}
	if msg == "" {
		return true
	}

	fmt.Fprintln(os.Stderr, "* Error: "+msg+"\n")
	return false
}

func main() {
	var (
		minQ, maxQ          int
		target              float64
		loops               int
		help, force, noCopy bool
	)

	flag.IntVar(&maxQ, "max", 95, "Maximum quality")
	flag.IntVar(&minQ, "min", 40, "Minimum quality")
	flag.Float64Var(&target, "t", 0.99995, "Set the target SSIM")
	flag.IntVar(&loops, "l", 6, "Maximum number of attempts to find the best quality")
	flag.BoolVar(&help, "h", false, "Print this help message")
	flag.Bool("f", false, "Overwrite the output image if it already exists")
	flag.Bool("c", false, "Disable copying files that will not be compressed")
	flag.Parse()

	src, dest := flag.Arg(0), flag.Arg(1)

	for _, n := range os.Args {
		if n == "-f" {
			force = true
		} else if n == "-c" {
			noCopy = true
		}
	}

	flag.Usage = func() {
		fmt.Fprintln(os.Stderr, "Usage: ./jpeg-recompress src dest [options]")
		fmt.Fprintln(os.Stderr, "All metadata will be lost during this process")
		fmt.Fprintln(os.Stderr, "If no match is found, the original webp image will be copied over, otherwise it will use the quality that produces the lowest and closest size to the original")
		fmt.Fprintln(os.Stderr, "")
		fmt.Fprintln(os.Stderr, "Options:")
		flag.PrintDefaults()
	}

	if help {
		flag.Usage()
		return
	}

	if !checkArgs(src, dest, force, maxQ, minQ, target, loops) {
		flag.Usage()
		os.Exit(1)
	}

	original, err := readImage(src)
	if err != nil {
		panic(err)
	}
	originalSize, err := getFilesize(src)
	originalGray := convertToGray(original)
	if err != nil {
		panic(err)
	}
	fmt.Printf("Original Size = %.2fKB\n", float32(originalSize)/1024)

	var bestSize = originalSize
	var bestQ int
	var bestIndex float64
	var fallbackQ int
	var fallbackSize int64
	var fallbackIndex float64
	for attempt := 1; attempt <= loops; attempt++ {
		var q = minQ + (maxQ-minQ)/2
		if minQ == maxQ {
			break
		}
		index, data, err := compare(originalGray, q)
		if err != nil {
			panic("Error when comparing images")
		}
		newSize := int64(len(data))
		fmt.Printf("[%v] Quality = %v, SSIM = %.5f, Size = %.2fKB\n", attempt, q, index, float32(newSize)/1024)

		if newSize >= originalSize {
			if index < target {
				attempt = loops
			} else {
				maxQ = int(math.Max(float64(q-1), float64(minQ)))
			}
		} else {
			if index < target {
				minQ = int(math.Min(float64(q+1), float64(maxQ)))
			} else if index > target {
				maxQ = int(math.Max(float64(q-1), float64(minQ)))
			} else {
				attempt = loops
			}
		}
		if newSize < bestSize && index >= target {
			bestSize = newSize
			bestQ = q
			bestIndex = index
		}

		if fallbackSize == 0 {
			fallbackSize = newSize
		}
		if newSize <= originalSize && newSize > fallbackSize {
			fallbackSize = newSize
			fallbackQ = q
			fallbackIndex = index
		} else if newSize > originalSize && newSize < fallbackSize {
			fallbackSize = newSize
			fallbackQ = q
			fallbackIndex = index
		}
	}

	if bestSize < originalSize {
		data, err := encodeToJPEGBytes(original, bestQ)
		if err != nil {
			panic(err)
		}
		save(dest, data)
		fmt.Printf("Final image:\nQuality = %v, SSIM = %.5f, Size = %.2fKB\n", bestQ, bestIndex, float32(bestSize)/1024)
		fmt.Printf("%.1f%% of original, saved %.2fKB", float32(bestSize)/float32(originalSize)*100, float32(originalSize-bestSize)/1024)
	} else {
		if noCopy {
			fmt.Println("* Can't find any match, not saving any image")
			return
		}
		if isJpeg(src) {
			fmt.Println("* Can't find any match, copying oringal image")
			_, err := copyFile(src, dest)
			if err != nil {
				panic(err)
			}
		} else {
			fmt.Println("* Can't find any match, falling back to closest match")
			fmt.Printf("Final image:\nQuality = %v, SSIM = %.5f, Size = %.2fKB\n", fallbackQ, fallbackIndex, float32(fallbackSize)/1024)
			fmt.Printf("%.1f%% of original, saved %.2fKB", float32(fallbackSize)/float32(originalSize)*100, float32(originalSize-fallbackSize)/1024)
			data, err := encodeToJPEGBytes(original, fallbackQ)
			if err != nil {
				panic(err)
			}
			save(dest, data)
		}
	}
}
