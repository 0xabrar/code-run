package e2e

import (
    "bytes"
    "encoding/json"
    "io/ioutil"
    "net/http"
    "testing"
    "time"

    "github.com/acme/coderunner/internal/models"
)

// Helper to submit a run request and poll until done
func runAndWait(t *testing.T, req models.RunRequest) []models.RunResult {
    t.Helper()

    body, _ := json.Marshal(req)
    resp, err := http.Post("http://localhost:8080/run", "application/json", bytes.NewReader(body))
    if err != nil {
        t.Fatalf("post /run: %v", err)
    }
    defer resp.Body.Close()
    var m map[string]string
    if err := json.NewDecoder(resp.Body).Decode(&m); err != nil {
        t.Fatalf("decode: %v", err)
    }
    statusURL := "http://localhost:8080" + m["status"]

    // poll
    for i := 0; i < 60; i++ {
        r, err := http.Get(statusURL)
        if err != nil {
            t.Fatalf("poll: %v", err)
        }
        if r.StatusCode == http.StatusNotFound {
            time.Sleep(1 * time.Second)
            continue
        }
        data, _ := ioutil.ReadAll(r.Body)
        r.Body.Close()
        var res []models.RunResult
        if err := json.Unmarshal(data, &res); err != nil {
            t.Fatalf("unmarshal: %v body=%s", err, string(data))
        }
        return res
    }
    t.Fatalf("timeout waiting for job")
    return nil
}

func TestContainerMostWater(t *testing.T) {
    tests := []models.TestCase{
        {Stdin: "1 8 6 2 5 4 8 3 7\n", Expected: "49\n"},
        {Stdin: "1 1\n", Expected: "1\n"},
    }

    goCode := `package main
import (
    "bufio"
    "fmt"
    "os"
)
func main(){
    in:=bufio.NewReader(os.Stdin)
    var arr []int
    for{var x int;if _,err:=fmt.Fscan(in,&x);err!=nil{break};arr=append(arr,x)}
    l,r,max:=0,len(arr)-1,0
    for l<r {
        h:=arr[l]
        if arr[r]<h {h=arr[r]}
        area:=h*(r-l)
        if area>max {max=area}
        if arr[l]<arr[r] {l++} else {r--}
    }
    fmt.Println(max)
}`

    pyCode := `import sys
heights=list(map(int,sys.stdin.read().split()))
l,r=0,len(heights)-1
max_area=0
while l<r:
    h=min(heights[l],heights[r])
    max_area=max(max_area,h*(r-l))
    if heights[l]<heights[r]:
        l+=1
    else:
        r-=1
print(max_area)`

    jsCode := `let data='';process.stdin.on('data',c=>data+=c).on('end',()=>{
  const h=data.trim().split(/\s+/).filter(Boolean).map(Number);
  let l=0,r=h.length-1,max=0;
  while(l<r){
    const height=Math.min(h[l],h[r]);
    max=Math.max(max,height*(r-l));
    if(h[l]<h[r]) l++; else r--;}
  console.log(max);
});`

    cases := []models.RunRequest{{Language:"go",Code:goCode,Tests:tests},{Language:"python",Code:pyCode,Tests:tests},{Language:"javascript",Code:jsCode,Tests:tests}}

    for _, c := range cases {
        res:=runAndWait(t,c)
        if len(res)!=len(tests){t.Fatalf("%s expected %d results got %d",c.Language,len(tests),len(res))}
        for i,r:=range res{
            if r.Stdout!=tests[i].Expected{
                t.Fatalf("%s test %d expected %q got %q stderr=%q",c.Language,i,tests[i].Expected,r.Stdout,r.Stderr)
            }
        }
    }
} 