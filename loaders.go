package main

import (
	"compress/gzip"
	"encoding/json"
	"fmt"
	"os"
)

const VecDim = 14

var referenceList = []RefData{}
var flatRefs []float32
var flatLabels []string
var normalizer = Normalizer{}
var mccrisk = make(map[string]float32)

func init() {
	if err := loadDataset(); err != nil {
		fmt.Println("Failed to load dataset")
		panic(err)
	}

	if err := loadNormalizer(); err != nil {
		fmt.Println("Failed to load normalizer")
		panic(err)
	}

	if err := loadMccRisk(); err != nil {
		fmt.Println("Failed to load mcc risk file")
		panic(err)
	}

	fmt.Println("All resources loaded in memory")
}

func loadDataset() error {
	file, err := os.Open("../resources/references.json.gz")
	if err != nil {
		return err
	}
	defer file.Close()

	gz, err := gzip.NewReader(file)
	if err != nil {
		return err
	}
	defer gz.Close()

	if err := json.NewDecoder(gz).Decode(&referenceList); err != nil {
		return err
	}

	// OPTIMIZATION: pack into flat layout, then release the original []RefData
	// so we don't carry both copies in RAM (the box only has 8 GB).
	n := len(referenceList)
	flatRefs = make([]float32, n*VecDim)
	flatLabels = make([]string, n)
	for i, r := range referenceList {
		copy(flatRefs[i*VecDim:(i+1)*VecDim], r.Vector)
		flatLabels[i] = r.Label
	}
	referenceList = nil

	fmt.Println("Dataset loaded:", n, "refs in flat layout")
	return nil
}

func loadNormalizer() error {
	file, err := os.Open("../resources/normalization.json")
	if err != nil {
		return err
	}
	defer file.Close()

	if err := json.NewDecoder(file).Decode(&normalizer); err != nil {
		return err
	}

	fmt.Println("Normalizer File Loaded")
	return nil
}

func loadMccRisk() error {
	file, err := os.Open("../resources/mcc_risk.json")
	if err != nil {
		return err
	}
	defer file.Close()

	if err := json.NewDecoder(file).Decode(&mccrisk); err != nil {
		return err
	}

	fmt.Println("Mcc Risk File Loaded")
	return nil
}
