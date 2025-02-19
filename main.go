package main

import (
	"context"
	"math/rand"
	"time"

	"github.com/edaniels/golog"
	"github.com/google/uuid"
	"github.com/pkg/errors"
	"github.com/schollz/progressbar/v3"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.viam.com/utils"
)

const (
	mongoURI                       = "mongodb://localhost:27017/"
	QueryableTabularDatabaseName   = "sensorData"
	QueryableTabularCollectionName = "readings"
)

var logger = golog.NewDebugLogger("adf_provisioning script")

type arguments struct {
	OrgID            string `flag:"org_id,usage=org id"`
	LocID            string `flag:"loc_id,usage=location id"`
	MachineID        string `flag:"machine_id,usage=machine id"`
	PartID           string `flag:"part_id,usage=part id"`
	StartTime        string `flag:"start_time,usage=start date format:2006-01-02 15:04:05,required=true"`
	EndTime          string `flag:"end_time,usage=end date of data range, if left unset will be today format:2006-01-02 15:04:05"`
	IsMovementSensor bool   `flag:"is_mov,usage=UNIMPLEMENTED"`
	// IsMovementSensor bool   `flag:"is_mov,usage=if false will generate sensor data, if true will generate movement sensor"`
	Frequency int `flag:"f,usage=frequency of simulated data in hz,required=true"`
}

type classyInput struct {
	OrgID            string
	LocID            string
	MachineID        string
	PartID           string
	StartTime        time.Time
	EndTime          time.Time
	IsMovementSensor bool
	Frequency        int
}

func main() {
	utils.ContextualMain(mainWithArgs, logger)
}

func mainWithArgs(ctx context.Context, args []string, logger golog.Logger) error {
	var argsParsed arguments
	if err := utils.ParseFlags(args, &argsParsed); err != nil {
		return err
	}
	orgID := getValueOrGenerateRandom(argsParsed.OrgID)
	locID := getValueOrGenerateRandom(argsParsed.LocID)
	machineID := getValueOrGenerateRandom(argsParsed.MachineID)
	partID := getValueOrGenerateRandom(argsParsed.PartID)

	mongoClient, err := mongo.Connect(ctx, options.Client().ApplyURI(mongoURI))
	if err != nil {
		return errors.Wrap(err, "failed to connect to local mongo")
	}
	defer mongoClient.Disconnect(ctx)
	coll := mongoClient.Database(QueryableTabularDatabaseName).Collection(orgID)

	var endTime time.Time
	if argsParsed.EndTime == "" {
		endTime = time.Now()
	} else {
		end, err := time.Parse(time.DateTime, argsParsed.EndTime)
		if err != nil {
			return errors.WithMessagef(err, "failed to parse provided end time, please use format %s. Provided input:",
				time.DateTime, argsParsed.EndTime)
		}
		endTime = end
	}

	startTime, err := time.Parse(time.DateTime, argsParsed.StartTime)
	if err != nil {
		return errors.WithMessagef(err, "failed to parse provided start time, please use format %s. Provided input:",
			time.DateTime, argsParsed.EndTime)
	}

	numDatapoints := calculateNumDataPoints(argsParsed.Frequency, startTime, endTime)

	logger.Infof("Will generate %d datapoints between %s and %s", numDatapoints, startTime, endTime)

	input := classyInput{
		OrgID:            orgID,
		LocID:            locID,
		MachineID:        machineID,
		PartID:           partID,
		StartTime:        startTime,
		EndTime:          endTime,
		IsMovementSensor: argsParsed.IsMovementSensor,
		Frequency:        argsParsed.Frequency,
	}

	return generateDatapoints(ctx, input, coll, numDatapoints)
}

// {
//     "capture_day": "2024-08-28 00:00:00 +0000 UTC",
//     "organization_id": "bf92eb43-866c-466b-ba14-bcbe986a689d",
//     "data": {
//       "lengths_mm": [
//         5000
//       ]
//     },
//     "component_type": "rdk:component:gantry",
//     "method_name": "Lengths",
//     "additional_parameters": {},
//     "robot_id": "fe0e2671-837e-4650-ae89-ac302bd8399a",
//     "part_id": "7e4ff19f-40ef-4dbf-9151-659152cfb65a",
//     "component_name": "gantry-1",
//     "time_received": "2024-08-28 16:02:39.907 +0000 UTC",
//     "tags": null,
//     "location_id": "yqphsi3r1e",
//     "time_requested": "2024-08-28 16:02:39.907 +0000 UTC"
//   },

type nullObject struct{}

type movementSensor struct {
}

type sensor struct {
	Readings sensorReadings `bson:"readings"`
}

type sensorReadings struct {
	ViamUploaded bool    `bson:"viam_uploaded"`
	Time         string  `bson:"time"`
	Type         string  `bson:"type"`
	Temp         float64 `bson:"temp"`
	CookTime     float64 `bson:"cook_time"`
	BeginTime    float64 `bson:"begin_time"`
	// "viam_uploaded": "0",
	// "time": "2024-09-17 20:55:44",
	// "type": "original",
	// "temp": "2590.34",
	// "cook_time": "184.38",
	// "begin_time": "1726610000.0"
}

type datapoint struct {
	OrgID                string      `bson:"organization_id"`
	LocID                string      `bson:"location_id"`
	RobotID              string      `bson:"robot_id"`
	PartID               string      `bson:"part_id"`
	ComponentName        string      `bson:"component_name"`
	ComponentType        string      `bson:"component_type"`
	MethodName           string      `bson:"method_name"`
	Tags                 *nullObject `bson:"tags"`
	AdditionalParameters nullObject  `bson:"additional_parameters"`
	Data                 any         `bson:"data"`
	CaptureDay           time.Time   `bson:"capture_day"`
	TimeRequested        time.Time   `bson:"time_requested"`
	TimeReceived         time.Time   `bson:"time_received"`
}

func generateDatapoints(ctx context.Context, input classyInput, coll *mongo.Collection, numDatapoints int) error {
	logger.Info("Generating data points...")
	bar := progressbar.Default(int64(numDatapoints))

	period := 1.0 / input.Frequency

	docs := []any{}

	componentName := "sensy-1"
	componentType := "rdk:component:sensor"
	methodName := "Readings"
	if input.IsMovementSensor {
		componentName = "movie-1"
		componentType = "rdk:component:movement_sensor"
		methodName = ""
	}

	for iter := input.StartTime; !iter.After(input.EndTime); iter = iter.Add(time.Duration(period) * time.Second) {
		bar.Add(1)
		sensorReading := sensor{
			Readings: sensorReadings{
				ViamUploaded: false,
				Time:         generateRandomString(100_00),
				Type:         generateRandomString(100_00),
				Temp:         randFloatN(500),
				CookTime:     randFloatN(200),
				BeginTime:    float64(iter.Unix()),
			},
		}
		docs = append(docs, datapoint{
			OrgID:         input.OrgID,
			LocID:         input.LocID,
			RobotID:       input.MachineID,
			PartID:        input.PartID,
			ComponentName: componentName,
			ComponentType: componentType,
			MethodName:    methodName,
			TimeRequested: iter,
			TimeReceived:  iter,
			CaptureDay:    floorTime(iter),
			Data:          sensorReading,
		})
	}

	_, err := coll.InsertMany(ctx, docs)
	if err != nil {
		return errors.Wrap(err, "error inserting data to mongo")
	}

	return nil

}

func randFloatN(n float64) float64 {
	return rand.Float64() * n
}

func floorTime(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
}

func calculateNumDataPoints(freq int, startTime, endTime time.Time) int {
	timeSpan := endTime.Sub(startTime)
	return int(timeSpan.Minutes()) * freq * 60
}

func getValueOrGenerateRandom(arg string) string {
	if arg == "" {
		return uuid.New().String()
	}
	return arg
}

func generateRandomString(length int) string {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	b := make([]byte, length)
	for i := range b {
		b[i] = charset[rand.Intn(len(charset))]
	}
	return string(b)
}
