el problema que que queremos resolver:
    
    - aplicación para compilar varios main en Go dentro de un directorio, pero tengo la inquietud de cómo puedo saber qué archivos son los que pertenece a cada main al momento de compilar, ya se identificar el archivo modificado pero no exactamente a que main esta asociado, por ejemplo Tengo un main que es un servidor web, otro una aplicación cmd de línea de comando y tengo otro main para compilar a webassembly. pero algunos comparten ciertos paquetes 
    
    Entonces cuando yo modifico un archivo que pertenece a un paquete x y Estos son compartidos por uno de los tres main a compilar, Cómo puedo saber a qué main pertenece una vez que fue modificado. 

para resolverlo con el test: 

- creando un método publico de godepfind llamado GoFileComesFromMain(fileName string) []string donde por parámetro se reciba el nombre del archivo modificado ej: "module3.go" y retorne los nombres de los main al que pertenece, en caso de no pertenecer a ninguno retornar slice vacío.


>funcion para usar solo en el test y obtener el nombre del fichero
```go
// getFileName returns the filename from a path
// Example: "theme/index.html" -> "index.html"
func getFileName(path string) (string, error) {
	if path == "" {
		return "", errors.New("GetFileName empty path")
	}

	// Check if path ends with a separator
	if len(path) > 0 && (path[len(path)-1] == '/' || path[len(path)-1] == '\\') {
		return "", errors.New("GetFileName invalid path: ends with separator")
	}

	fileName := filepath.Base(path)
	if fileName == "." || fileName == string(filepath.Separator) {
		return "", errors.New("GetFileName invalid path")
	}
	if len(path) > 0 && path[len(path)-1] == filepath.Separator {
		return "", errors.New("GetFileName invalid path")
	}

	return fileName, nil
}
```

 
 - para el test (usando directorio temporal test) deberia tener 3 carpetas appAserver, appBcmd y appCwasm dentro de cada una de ellas solo main.go . otra carpeta llamada modules y dentro de ella otras sub carpetas module1, module2, module3, module4.. dentro de cada carpeta solo un fichero llamado igual que el modulo ej module3.go y dentro de cada fichero solo una funcion exportada basica..

 la ide es que las app importen algunos modulos y alguno se combinen entre ella y otros no solo uno 