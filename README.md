A tool for inserting mock ADF data based on a date range. Currently this is hardcoded for particular type of sensor reading.

# Usage
The only required argument is start time. If end time is left blank, it will become `time.now()`.
All ids can be left blank. If so, they will become random UUIDs. I reccomend setting them to whatever robot you have locally, this way you can use the open as query button as expected.
## Args
-start_time string start date format:2006-01-02 15:04:05

-end_time string end date of data range

-f int frequency of simulated data in hz

-loc_id string location id

-machine_id string machine id

-org_id string org id

-part_id string part id

## Example command

`go run main.go -org_id "67981403-9f84-431e-b467-a719cd47771d" -loc_id "38w7k9q9pa" -machine_id "4aadcd6a-e0ee-4133-aa94-46ae6acd048f" -part_id "87e82e15-c8ff-457c-af31-3ecc149f8369" -start_time "2024-10-14 01:04:05" -f 1`
