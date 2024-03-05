package main

// Import statements (conslidated)
import (
	"os"
	"encoding/json"
	"strings"
	"io"
	"context"
    "fmt"
    "log"
    "net/http"
    "time"

    "go.mongodb.org/mongo-driver/bson"
    "go.mongodb.org/mongo-driver/mongo"
    "go.mongodb.org/mongo-driver/mongo/options"
)

// Single Student strct definition with BSON tags 
type Student struct {
    Name      string `bson:"name" json:"name"`
    Age       int    `bson:"age" json:"age"`
    StudentID string `bson:"student_id" json:"student_id"`
}

var client *mongo.Client

func init() {
    var err error
    // Connect to MongoDB
    client, err = mongo.NewClient(options.Client().ApplyURI("mongodb://localhost:27017"))
    if err != nil {
        log.Fatal(err)
    }
    ctx, _ := context.WithTimeout(context.Background(), 10*time.Second)
    err = client.Connect(ctx)
    if err != nil {
        log.Fatal(err)
    }
    // Check the connection
    err = client.Ping(ctx, nil)
    if err != nil {
        log.Fatal(err)
    }

    fmt.Println("Connected to MongoDB!")
}

func AddStudent(student Student) error {
    collection := client.Database("studentDB").Collection("students")
    ctx, _ := context.WithTimeout(context.Background(), 5*time.Second)
    _, err := collection.InsertOne(ctx, student)
    return err
}


// represents the main persistent data structure. 
type Student struct {
	Name 		string `json:"name"`
	Age 		int 	`json:"age"` 
	StudentID 	string // `json:"student_id"`  try to auto increment 
}

// studentDB is the handle to students in-memory storage. 
type studentsDB struct {
	students map[string]Student 
	lastStudentID int // New field to track the last StudentID 
}

//Add adds new students to the storage. 
func (db *studentsDB) Add(s Student) error {
	db.lastStudentID++ // Increment the lastStudentID
	s.StudentID = fmt.Sprintf("%d", db.lastStudentID)//Assign the StudentID to the student 
	db.students[s.StudentID] = s  // Add the student to the map 
	return nil 
}
// Count returns the current count fo the students in in-memory storage. 
func(db *studentsDB) Count() int {
	return len(db.students)
}
// Get returns a student with a given ID or empty student struct. 
func (db *studentsDB) Get(keyID string) (Student, bool) {
	s, ok := db.students[keyID]
	return s, ok 
}
// GetAll returns all the students  as slice 
func (db *studentsDB) GetAll() []Student {
	all := make([]Student, 0, db.Count())
	for _, s := range db.students {
		all = append(all, s)
	}
	return all 
}
// Remove removes student with given keyID from database
func (db studentsDB) Remove(keyID string) bool {
	// Assessing whether student actually exists 
	_, res := db.Get(keyID)
	// Deleting entry either way 
	delete(db.students, keyID) 
	return res 
}


// represents a unified way of acccessing Student data. 
type StudentsStorage interface {
	Add(s Student) error 
	Count() int 
	Get(key string) (Student, bool)
	GetAll() []Student
	Remove(key string) bool 
}





//Initialises the studentsstorage. 
func InitStudentsStorage() StudentsStorage {
	db := studentsDB{}
	db.students = make(map[string]Student)

	// Prepopulate with DATA 
	s1 := Student{Name: "Tungno", Age: 30}
	s2 := Student{Name: "Didim", Age: 31}
	s3 := Student{Name: "Vaanpi", Age: 4}
	s4 := Student{Name: "Ompi", Age: 2}	

	db.Add(s1)
	db.Add(s2)
	db.Add(s3)
	db.Add(s4)
	log.Println("Prepolated DB...")

	return &db
}
// handleStudentPost utility function, package level, for handling POSt request 
func handleStudentPost(w http.ResponseWriter, r *http.Request, db StudentsStorage) {
	// Check if the request body is empty
	if r.Body == nil {
		http.Error(w, "No data to process", http.StatusBadRequest)
		return 
	}

	//This function retrieves data from the request and adds a new student based on the data.
	var s Student
	err := json.NewDecoder(r.Body).Decode(&s)
	if err == io.EOF {
		// This error occurs if the body is empty
		http.Error(w, "No data in POST request", http.StatusBadRequest)
		return 
	} else if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return 
	}

	// Check whether student is properly populated
	/*
	if s.StudentID == "" {
		http.Error(w, "Input did not contain student ID. Recheck posted student information.", http.StatusBadRequest)
		fmt.Println("Empty ID on student:", s)
		return
	}
	*/

	// check if the student is new
	_, ok := db.Get(s.StudentID)
	if ok {
		http.Error(w, "Student already exists. Use PUT to modify.", http.StatusBadRequest)
		fmt.Println("Student already exists.")
		return
	}
	// new student
	fmt.Println("Adding student to db ...")
	err = db.Add(s)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		log.Println("Error when adding student to DB:", http.StatusInternalServerError)
		return
	}
	 w.WriteHeader(http.StatusOK) // Proper way to set status code
    fmt.Fprintf(w, "Student added successfully")
}

// replyWithAllStudents prepares a response with all students from the student storage
func replyWithAllStudents(w io.Writer, db StudentsStorage) {
	if db.Count() == 0 {
		err := json.NewEncoder(w).Encode([]Student{})
		if err != nil {
			// this should never happen
			fmt.Println("ERROR encoding JSON for an empty array", err)
		}
	} else {
		a := make([]Student, 0, db.Count())
		a = append(a, db.GetAll()...)
		err := json.NewEncoder(w).Encode(a)
		if err != nil {
			fmt.Println("ERROR encoding JSON", err)
		}
	}
}

// replyWithStudent prepares a response with a single student from the student storage
func replyWithStudent(w http.ResponseWriter, db StudentsStorage, id string) {
	// make sure that i is valid
	s, ok := db.Get(id)
	if !ok {
		http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
		return
	}
	// handle /student/<id>
	err := json.NewEncoder(w).Encode(s)
	if err != nil {
		fmt.Println("ERROR encoding JSON", err)
	}
}

// handleStudentGet utility function, package level, to handle GET request to student route
func handleStudentGet(w http.ResponseWriter, r *http.Request, db StudentsStorage) {
	// This function returns all students/single student based on the received request form. If it contains ID then a single one
	http.Header.Add(w.Header(), "content-type", "application/json")
	// alternative way:
	// w.Header().Add("content-type", "application/json")
	parts := strings.Split(r.URL.Path, "/")
	// error handling
	if len(parts) != 3 || parts[1] != "students" {
		http.Error(w, "Malformed URL", http.StatusBadRequest)
		return
	}
	// handle the request /students/ which will return ALL students as array of JSON objects
	if parts[2] == "" {
		replyWithAllStudents(w, db)
	} else {
		replyWithStudent(w, db, parts[2])
	}
}


// HandlerStudent main handler for route related to requests to /students 
func HandlerStudent(db StudentsStorage) func(http.ResponseWriter, *http.Request) {

	return func(w http.ResponseWriter, r *http.Request) {
		 log.Printf("Received %s request for %s", r.Method, r.URL.Path)

		switch r.Method {
		case http.MethodPost:
			handleStudentPost(w, r, db)
			return 
		case http.MethodGet: 
			handleStudentGet(w,r,db)
			return 
		default:
			http.Error(w, "Method "+r.Method+" not supported.", http.StatusMethodNotAllowed)
			return 
		}
	}
}


// Middleware to add CORS headers
func enableCORS(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        // Set CORS headers
        w.Header().Set("Access-Control-Allow-Origin", "*") // Be more specific in production code
        w.Header().Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS, PUT, DELETE")
        w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Accept")

        // Check if it's a preflight request and handle it.
        if r.Method == http.MethodOptions {
            w.WriteHeader(http.StatusOK)
            return
        }

        // Process the next handler if it's not a preflight request
        next.ServeHTTP(w, r)
    })
}

// main function
func main() {
	port := os.Getenv("PORT")
	if port == "" {
		log.Println("$PORT has not been set. Default: 8080")
		port = "8080"
	}
	// DB init
	db := InitStudentsStorage()

	mainHandler := http.HandlerFunc(HandlerStudent(db))
    wrappedHandler := enableCORS(mainHandler)

    http.Handle("/students/", wrappedHandler)
	

	


	// handler function 
	//http.HandleFunc("/students/", HandlerStudent(db))


	log.Println("Listening on port " + port)
	log.Fatal(http.ListenAndServe(":"+port, nil))

}