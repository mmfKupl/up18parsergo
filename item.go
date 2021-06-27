package parser

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
)

const (
	startFileLength = len("{\n  \"mappedParsedData\": [\n")
	endFileLength   = len("\n  ]\n}")
)

func AppendItemToFile(item Item, file *os.File) error {
	items := make([]Item, 1)
	items[0] = item

	itemsMap := map[string][]Item{
		"mappedParsedData": items,
	}

	jsonItems, err := json.MarshalIndent(itemsMap, "", "  ")
	if err != nil {
		return err
	}

	fileInfo, err := file.Stat()
	if err != nil {
		return fmt.Errorf("failed to read file info: %s", err)
	}

	if fileInfo.Size() == 0 {
		_, err = file.Write(jsonItems)
		return err
	}

	jsonItems = jsonItems[startFileLength:]

	_, err = file.WriteAt(append([]byte(",\n"), jsonItems...), fileInfo.Size()-int64(endFileLength))
	return err
}

func AppendUnparsedItemToFile(item Item) {
	filePath := GetValidPath("__crushed-items.json")
	file, err := CreateAndGetFile(filePath, os.O_WRONLY)
	if err != nil {
		fmt.Printf("Не удалось открыть новый файл.\n")
		return
	}

	err = AppendItemToFile(item, file)
	if err != nil {
		fmt.Printf("Не удалось записать неудавшиеся данные в новый файл: %s, %s: %s\n", item.GetId(), item.GetLink(), err)
	}
}

func WriteCrushedUrlToFile(url string) {
	filePath := GetValidPath("__crushed-urls.json")
	file, err := CreateAndGetFile(filePath, os.O_RDWR)
	if err != nil {
		fmt.Printf("Не удалось открыть файл с незагруженными ссылками: %s.\n", err)
		return
	}
	defer file.Close()

	fileInfo, err := file.Stat()
	if err != nil {
		fmt.Printf("Не удалось прочитать файл с незагруженными ссылками: %s.\n", err)
		return
	}

	urls := make([]string, 0)

	if fileInfo.Size() != 0 {
		bytes := make([]byte, fileInfo.Size())
		_, err := file.Read(bytes)
		if err != nil {
			fmt.Printf("Не удалось прочитать файл с незагруженными ссылками: %s.\n", err)
			return
		}
		err = json.Unmarshal(bytes, &urls)
		if err != nil {
			fmt.Printf("Не удалось распарсить файл с незагруженными ссылками: %s.\n", err)
			return
		}
	}

	urls = append(urls, url)
	jsonUrls, err := json.MarshalIndent(urls, "", "  ")
	if err != nil {
		fmt.Printf("Не удалось преобразовать в json незагруженные ссылки: %s.\n", err)
		return
	}

	_, err = file.Seek(0, io.SeekStart)
	if err != nil {
		fmt.Printf("Неудалось очистить файл (1) `%s`: %s\n", filePath, err)
		return
	}
	err = file.Truncate(0)
	if err != nil {
		fmt.Printf("Неудалось очистить файл (2) `%s`: %s\n", filePath, err)
		return
	}
	_, err = file.Write(jsonUrls)
	if err != nil {
		fmt.Printf("Не удалось сохранить в файл незагруженные ссылки: %s.\n", err)
		return
	}
}
