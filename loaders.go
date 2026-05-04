package main

import (
	"compress/gzip"
	"encoding/json"
	"fmt"
	"os"
)

const VecDim = 14

var referenceList = []RefData{}
var ivfIndex *IVFIndex
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
	flat := make([]float32, n*VecDim)
	labels := make([]string, n)
	for i, r := range referenceList {
		copy(flat[i*VecDim:(i+1)*VecDim], r.Vector)
		labels[i] = r.Label
	}
	referenceList = nil

	ivfIndex = BuildIVF(flat, labels, n)

	fmt.Println("Dataset loaded:", n, "refs into IVF index (K=", IVFK, ")")
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
