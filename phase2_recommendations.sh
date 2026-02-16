#!/bin/bash
set -e
echo "FinOpsMind Phase 2: Building Recommendation Engine..."
mkdir -p backend/internal/recommendations/rules backend/internal/handler backend/migrations

# types.go
cat > backend/internal/recommendations/types.go << 'GOEOF'
package recommendations
import ("encoding/json";"time")
type RecommendationType string
const (TypeIdleResource RecommendationType="idle_resource";TypeUnattachedResource RecommendationType="unattached_resource";TypeOversized RecommendationType="oversized";TypeMissingOptimization RecommendationType="missing_optimization";TypeNetworkingWaste RecommendationType="networking_waste";TypeRetentionCleanup RecommendationType="retention_cleanup")
type Severity string
const (SeverityLow Severity="low";SeverityMedium Severity="medium";SeverityHigh Severity="high";SeverityCritical Severity="critical")
type Confidence string
const (ConfidenceLow Confidence="low";ConfidenceMedium Confidence="medium";ConfidenceHigh Confidence="high")
type ResourceMetadata struct{AccountID string`json:"account_id,omitempty"`;Region string`json:"region,omitempty"`;Tags map[string]string`json:"tags,omitempty"`;CurrentConfig map[string]interface{}`json:"current_config,omitempty"`;RecommendedConfig map[string]interface{}`json:"recommended_config,omitempty"`;Metrics map[string]float64`json:"metrics,omitempty"`;AnalysisPeriod string`json:"analysis_period,omitempty"`}
type Recommendation struct{ID string`json:"id"db:"id"`;Type RecommendationType`json:"type"db:"type"`;RuleID string`json:"rule_id"db:"rule_id"`;ResourceID string`json:"resource_id"db:"resource_id"`;ResourceType string`json:"resource_type"db:"resource_type"`;ResourceARN string`json:"resource_arn,omitempty"db:"resource_arn"`;AccountID string`json:"account_id"db:"account_id"`;Region string`json:"region"db:"region"`;CurrentState string`json:"current_state"db:"current_state"`;RecommendedAction string`json:"recommended_action"db:"recommended_action"`;EstimatedSavings float64`json:"estimated_savings"db:"estimated_savings"`;Confidence Confidence`json:"confidence"db:"confidence"`;Severity Severity`json:"severity"db:"severity"`;TerraformCode string`json:"terraform_code,omitempty"db:"terraform_code"`;ResourceMetadata json.RawMessage`json:"resource_metadata,omitempty"db:"resource_metadata"`;Status string`json:"status"db:"status"`;CreatedAt time.Time`json:"created_at"db:"created_at"`;UpdatedAt time.Time`json:"updated_at"db:"updated_at"`;ExpiresAt*time.Time`json:"expires_at,omitempty"db:"expires_at"`}
type Rule interface{ID()string;Name()string;Description()string;ResourceTypes()[]string;Evaluate(ctx*RuleContext)([]Recommendation,error)}
type RuleContext struct{DB DBQuerier;MetricsStore MetricsQuerier;PricingData PricingQuerier;Now time.Time;AccountIDs[]string;Regions[]string}
type DBQuerier interface{Query(query string,args...interface{})(Rows,error);QueryRow(query string,args...interface{})Row}
type Rows interface{Next()bool;Scan(dest...interface{})error;Close()error;Err()error}
type Row interface{Scan(dest...interface{})error}
type MetricsQuerier interface{GetAverageMetric(resourceID,metricName string,days int)(float64,error);GetMaxMetric(resourceID,metricName string,days int)(float64,error);GetSumMetric(resourceID,metricName string,days int)(float64,error)}
type PricingQuerier interface{GetEC2HourlyPrice(instanceType,region string)(float64,error);GetRDSHourlyPrice(instanceClass,engine,region string,multiAZ bool)(float64,error);GetEBSMonthlyPrice(volumeType,region string,sizeGB int)(float64,error);GetNATGatewayHourlyPrice(region string)(float64,error);GetDataTransferPrice(region string)(float64,error)}
type RuleResult struct{RuleID string`json:"rule_id"`;RuleName string`json:"rule_name"`;Recommendations[]Recommendation`json:"recommendations"`;Error error`json:"error,omitempty"`;Duration time.Duration`json:"duration"`}
type EngineResult struct{RunID string`json:"run_id"`;StartTime time.Time`json:"start_time"`;EndTime time.Time`json:"end_time"`;TotalDuration time.Duration`json:"total_duration"`;RulesExecuted int`json:"rules_executed"`;RulesFailed int`json:"rules_failed"`;TotalRecommendations int`json:"total_recommendations"`;TotalEstimatedSavings float64`json:"total_estimated_savings"`;Results[]RuleResult`json:"results"`}
func NewResourceMetadata(accountID,region string,tags map[string]string)ResourceMetadata{return ResourceMetadata{AccountID:accountID,Region:region,Tags:tags}}
func(m ResourceMetadata)ToJSON()json.RawMessage{data,_:=json.Marshal(m);return data}
func CalculateSeverity(monthlySavings float64)Severity{switch{case monthlySavings>=1000:return SeverityCritical;case monthlySavings>=500:return SeverityHigh;case monthlySavings>=100:return SeverityMedium;default:return SeverityLow}}
func GenerateRecommendationID(ruleID,resourceID string)string{return ruleID+"-"+resourceID+"-"+time.Now().Format("20060102")}
GOEOF

# engine.go
cat > backend/internal/recommendations/engine.go << 'GOEOF'
package recommendations
import("context";"encoding/json";"fmt";"log";"sync";"time";"github.com/google/uuid")
type Engine struct{rules[]Rule;db DBQuerier;metricsStore MetricsQuerier;pricingData PricingQuerier;mu sync.RWMutex;logger*log.Logger}
func NewEngine(db DBQuerier,metrics MetricsQuerier,pricing PricingQuerier)*Engine{return&Engine{rules:make([]Rule,0),db:db,metricsStore:metrics,pricingData:pricing,logger:log.Default()}}
func(e*Engine)RegisterRule(rule Rule){e.mu.Lock();defer e.mu.Unlock();e.rules=append(e.rules,rule)}
func(e*Engine)RegisterRules(rules...Rule){for _,rule:=range rules{e.RegisterRule(rule)}}
func(e*Engine)GetRules()[]Rule{e.mu.RLock();defer e.mu.RUnlock();return append([]Rule{},e.rules...)}
func(e*Engine)GetRuleByID(id string)(Rule,bool){e.mu.RLock();defer e.mu.RUnlock();for _,rule:=range e.rules{if rule.ID()==id{return rule,true}};return nil,false}
func(e*Engine)RunAllRules(ctx context.Context,opts...RunOption)(*EngineResult,error){runOpts:=&runOptions{concurrency:4};for _,opt:=range opts{opt(runOpts)};e.mu.RLock();rulesToRun:=e.filterRules(runOpts.ruleIDs);e.mu.RUnlock();result:=&EngineResult{RunID:uuid.New().String(),StartTime:time.Now(),Results:make([]RuleResult,0,len(rulesToRun))};ruleCtx:=&RuleContext{DB:e.db,MetricsStore:e.metricsStore,PricingData:e.pricingData,Now:time.Now(),AccountIDs:runOpts.accountIDs,Regions:runOpts.regions};resultChan:=make(chan RuleResult,len(rulesToRun));sem:=make(chan struct{},runOpts.concurrency);var wg sync.WaitGroup;for _,rule:=range rulesToRun{wg.Add(1);go func(r Rule){defer wg.Done();sem<-struct{}{};defer func(){<-sem}();select{case<-ctx.Done():resultChan<-RuleResult{RuleID:r.ID(),RuleName:r.Name(),Error:ctx.Err()};return;default:};resultChan<-e.executeRule(r,ruleCtx)}(rule)};go func(){wg.Wait();close(resultChan)}();for ruleResult:=range resultChan{result.Results=append(result.Results,ruleResult);result.RulesExecuted++;if ruleResult.Error!=nil{result.RulesFailed++}else{result.TotalRecommendations+=len(ruleResult.Recommendations);for _,rec:=range ruleResult.Recommendations{result.TotalEstimatedSavings+=rec.EstimatedSavings}}};result.EndTime=time.Now();result.TotalDuration=result.EndTime.Sub(result.StartTime);return result,nil}
func(e*Engine)executeRule(rule Rule,ctx*RuleContext)RuleResult{start:=time.Now();result:=RuleResult{RuleID:rule.ID(),RuleName:rule.Name()};defer func(){if r:=recover();r!=nil{result.Error=fmt.Errorf("panic:%v",r)};result.Duration=time.Since(start)}();recs,err:=rule.Evaluate(ctx);if err!=nil{result.Error=err;return result};result.Recommendations=recs;return result}
func(e*Engine)filterRules(ruleIDs[]string)[]Rule{if len(ruleIDs)==0{return e.rules};idSet:=make(map[string]bool);for _,id:=range ruleIDs{idSet[id]=true};filtered:=make([]Rule,0);for _,rule:=range e.rules{if idSet[rule.ID()]{filtered=append(filtered,rule)}};return filtered}
func(e*Engine)SaveRecommendations(ctx context.Context,recs[]Recommendation)error{if len(recs)==0{return nil};q:=`INSERT INTO recommendations(id,type,rule_id,resource_id,resource_type,resource_arn,account_id,region,current_state,recommended_action,estimated_savings,confidence,severity,terraform_code,resource_metadata,status,created_at,updated_at)VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18)ON CONFLICT(id)DO UPDATE SET estimated_savings=EXCLUDED.estimated_savings,terraform_code=EXCLUDED.terraform_code,updated_at=EXCLUDED.updated_at`;for _,r:=range recs{m,_:=json.Marshal(r.ResourceMetadata);_,err:=e.db.Query(q,r.ID,r.Type,r.RuleID,r.ResourceID,r.ResourceType,r.ResourceARN,r.AccountID,r.Region,r.CurrentState,r.RecommendedAction,r.EstimatedSavings,r.Confidence,r.Severity,r.TerraformCode,m,"pending",time.Now(),time.Now());if err!=nil{return err}};return nil}
type runOptions struct{accountIDs,regions,ruleIDs[]string;concurrency int}
type RunOption func(*runOptions)
func WithAccounts(ids...string)RunOption{return func(o*runOptions){o.accountIDs=ids}}
func WithRegions(r...string)RunOption{return func(o*runOptions){o.regions=r}}
func WithRules(ids...string)RunOption{return func(o*runOptions){o.ruleIDs=ids}}
func WithConcurrency(n int)RunOption{return func(o*runOptions){if n>0{o.concurrency=n}}}
GOEOF

# rules/base.go
cat > backend/internal/recommendations/rules/base.go << 'GOEOF'
package rules
import("fmt";"time";rec"github.com/finopsmind/backend/internal/recommendations")
type BaseRule struct{id,name,description string;resourceTypes[]string}
func(b*BaseRule)ID()string{return b.id}
func(b*BaseRule)Name()string{return b.name}
func(b*BaseRule)Description()string{return b.description}
func(b*BaseRule)ResourceTypes()[]string{return b.resourceTypes}
func newRec(ruleID,resourceID,resourceType,resourceARN,accountID,region string,recType rec.RecommendationType,currentState,action string,savings float64,conf rec.Confidence)rec.Recommendation{return rec.Recommendation{ID:rec.GenerateRecommendationID(ruleID,resourceID),Type:recType,RuleID:ruleID,ResourceID:resourceID,ResourceType:resourceType,ResourceARN:resourceARN,AccountID:accountID,Region:region,CurrentState:currentState,RecommendedAction:action,EstimatedSavings:savings,Confidence:conf,Severity:rec.CalculateSeverity(savings),Status:"pending",CreatedAt:time.Now(),UpdatedAt:time.Now()}}
const hoursPerMonth=730.0
func tfVal(v interface{})string{switch val:=v.(type){case string:return fmt.Sprintf(`"%s"`,val);case bool:return fmt.Sprintf("%t",val);default:return fmt.Sprintf("%v",val)}}
GOEOF

# rules/idle_ec2.go
cat > backend/internal/recommendations/rules/idle_ec2.go << 'GOEOF'
package rules
import("fmt";rec"github.com/finopsmind/backend/internal/recommendations")
type IdleEC2Rule struct{BaseRule;CPUThreshold float64;LookbackDays int}
func NewIdleEC2Rule()*IdleEC2Rule{return&IdleEC2Rule{BaseRule:BaseRule{id:"idle-ec2",name:"Idle EC2 Instances",description:"EC2 with <5% CPU for 14 days",resourceTypes:[]string{"aws_ec2_instance"}},CPUThreshold:5.0,LookbackDays:14}}
func(r*IdleEC2Rule)Evaluate(ctx*rec.RuleContext)([]rec.Recommendation,error){q:=fmt.Sprintf(`SELECT i.instance_id,i.instance_type,i.arn,i.account_id,i.region,COALESCE(m.avg_cpu,0),COALESCE(m.max_cpu,0)FROM ec2_instances i LEFT JOIN(SELECT resource_id,AVG(value)avg_cpu,MAX(value)max_cpu FROM cloudwatch_metrics WHERE metric_name='CPUUtilization'AND timestamp>NOW()-INTERVAL'%d days'GROUP BY resource_id)m ON i.instance_id=m.resource_id WHERE i.state='running'AND COALESCE(m.avg_cpu,0)<$1`,r.LookbackDays);rows,err:=ctx.DB.Query(q,r.CPUThreshold);if err!=nil{return nil,err};defer rows.Close();var recs[]rec.Recommendation;for rows.Next(){var id,itype,arn,acct,region string;var avgCPU,maxCPU float64;if err:=rows.Scan(&id,&itype,&arn,&acct,&region,&avgCPU,&maxCPU);err!=nil{continue};price,_:=ctx.PricingData.GetEC2HourlyPrice(itype,region);if price==0{price=0.10};conf:=rec.ConfidenceHigh;if maxCPU>20{conf=rec.ConfidenceMedium};r:=newRec(r.id,id,"ec2_instance",arn,acct,region,rec.TypeIdleResource,fmt.Sprintf("Running %s with %.1f%% avg CPU",itype,avgCPU),"Stop or terminate instance",price*hoursPerMonth,conf);r.TerraformCode=fmt.Sprintf("# Stop idle EC2: %s\n# aws ec2 stop-instances --instance-ids %s",id,id);recs=append(recs,r)};return recs,rows.Err()}
GOEOF

# rules/idle_rds.go
cat > backend/internal/recommendations/rules/idle_rds.go << 'GOEOF'
package rules
import("fmt";rec"github.com/finopsmind/backend/internal/recommendations")
type IdleRDSRule struct{BaseRule;CPUThreshold float64;ConnThreshold int;LookbackDays int}
func NewIdleRDSRule()*IdleRDSRule{return&IdleRDSRule{BaseRule:BaseRule{id:"idle-rds",name:"Idle RDS Instances",description:"RDS with <5% CPU and minimal connections",resourceTypes:[]string{"aws_rds_instance"}},CPUThreshold:5.0,ConnThreshold:5,LookbackDays:7}}
func(r*IdleRDSRule)Evaluate(ctx*rec.RuleContext)([]rec.Recommendation,error){q:=fmt.Sprintf(`SELECT d.db_instance_id,d.db_instance_class,d.engine,d.arn,d.account_id,d.region,d.multi_az,COALESCE(m.avg_cpu,0),COALESCE(m.avg_conn,0)FROM rds_instances d LEFT JOIN(SELECT resource_id,AVG(CASE WHEN metric_name='CPUUtilization'THEN value END)avg_cpu,AVG(CASE WHEN metric_name='DatabaseConnections'THEN value END)avg_conn FROM cloudwatch_metrics WHERE metric_name IN('CPUUtilization','DatabaseConnections')AND timestamp>NOW()-INTERVAL'%d days'GROUP BY resource_id)m ON d.db_instance_id=m.resource_id WHERE d.status='available'AND COALESCE(m.avg_cpu,0)<$1 AND COALESCE(m.avg_conn,0)<$2`,r.LookbackDays);rows,err:=ctx.DB.Query(q,r.CPUThreshold,r.ConnThreshold);if err!=nil{return nil,err};defer rows.Close();var recs[]rec.Recommendation;for rows.Next(){var id,class,engine,arn,acct,region string;var multiAZ bool;var avgCPU,avgConn float64;if err:=rows.Scan(&id,&class,&engine,&arn,&acct,&region,&multiAZ,&avgCPU,&avgConn);err!=nil{continue};price,_:=ctx.PricingData.GetRDSHourlyPrice(class,engine,region,multiAZ);if price==0{price=0.20};r:=newRec(r.id,id,"rds_instance",arn,acct,region,rec.TypeIdleResource,fmt.Sprintf("RDS %s with %.1f%% CPU, %.0f connections",class,avgCPU,avgConn),"Stop or delete RDS instance",price*hoursPerMonth,rec.ConfidenceHigh);r.TerraformCode=fmt.Sprintf("# Stop idle RDS: %s\n# aws rds stop-db-instance --db-instance-identifier %s",id,id);recs=append(recs,r)};return recs,rows.Err()}
GOEOF

# rules/idle_elb.go
cat > backend/internal/recommendations/rules/idle_elb.go << 'GOEOF'
package rules
import("fmt";rec"github.com/finopsmind/backend/internal/recommendations")
type IdleELBRule struct{BaseRule;RequestThreshold int;LookbackDays int}
func NewIdleELBRule()*IdleELBRule{return&IdleELBRule{BaseRule:BaseRule{id:"idle-elb",name:"Idle Load Balancers",description:"ALB/NLB/CLB with zero requests over 7 days",resourceTypes:[]string{"aws_lb","aws_elb"}},RequestThreshold:100,LookbackDays:7}}
func(r*IdleELBRule)Evaluate(ctx*rec.RuleContext)([]rec.Recommendation,error){q:=fmt.Sprintf(`SELECT lb.name,lb.arn,lb.type,lb.account_id,lb.region,COALESCE(m.total_requests,0)FROM load_balancers lb LEFT JOIN(SELECT resource_id,SUM(value)total_requests FROM cloudwatch_metrics WHERE metric_name IN('RequestCount','ProcessedBytes')AND timestamp>NOW()-INTERVAL'%d days'GROUP BY resource_id)m ON lb.arn=m.resource_id WHERE lb.state='active'AND COALESCE(m.total_requests,0)<$1`,r.LookbackDays);rows,err:=ctx.DB.Query(q,r.RequestThreshold);if err!=nil{return nil,err};defer rows.Close();var recs[]rec.Recommendation;for rows.Next(){var name,arn,lbType,acct,region string;var totalReq float64;if err:=rows.Scan(&name,&arn,&lbType,&acct,&region,&totalReq);err!=nil{continue};r:=newRec(r.id,name,"load_balancer",arn,acct,region,rec.TypeIdleResource,fmt.Sprintf("%s '%s' with %.0f requests in %d days",lbType,name,totalReq,r.LookbackDays),"Delete unused load balancer",22.0,rec.ConfidenceHigh);recs=append(recs,r)};return recs,rows.Err()}
GOEOF

# rules/unattached_ebs.go
cat > backend/internal/recommendations/rules/unattached_ebs.go << 'GOEOF'
package rules
import("fmt";rec"github.com/finopsmind/backend/internal/recommendations")
type UnattachedEBSRule struct{BaseRule;MinDaysUnattached int}
func NewUnattachedEBSRule()*UnattachedEBSRule{return&UnattachedEBSRule{BaseRule:BaseRule{id:"unattached-ebs",name:"Unattached EBS Volumes",description:"EBS volumes in available state",resourceTypes:[]string{"aws_ebs_volume"}},MinDaysUnattached:7}}
func(r*UnattachedEBSRule)Evaluate(ctx*rec.RuleContext)([]rec.Recommendation,error){q:=fmt.Sprintf(`SELECT v.volume_id,v.arn,v.account_id,v.region,v.volume_type,v.size_gb FROM ebs_volumes v WHERE v.state='available'AND v.created_at<NOW()-INTERVAL'%d days'`,r.MinDaysUnattached);rows,err:=ctx.DB.Query(q);if err!=nil{return nil,err};defer rows.Close();var recs[]rec.Recommendation;for rows.Next(){var id,arn,acct,region,vType string;var sizeGB int;if err:=rows.Scan(&id,&arn,&acct,&region,&vType,&sizeGB);err!=nil{continue};cost:=float64(sizeGB)*0.10;if vType=="gp3"{cost=float64(sizeGB)*0.08};r:=newRec(r.id,id,"ebs_volume",arn,acct,region,rec.TypeUnattachedResource,fmt.Sprintf("Unattached %s volume (%dGB)",vType,sizeGB),"Snapshot and delete volume",cost,rec.ConfidenceHigh);r.TerraformCode=fmt.Sprintf("# Delete unattached EBS: %s\n# aws ec2 create-snapshot --volume-id %s\n# aws ec2 delete-volume --volume-id %s",id,id,id);recs=append(recs,r)};return recs,rows.Err()}
GOEOF

# rules/unattached_eip.go
cat > backend/internal/recommendations/rules/unattached_eip.go << 'GOEOF'
package rules
import("fmt";rec"github.com/finopsmind/backend/internal/recommendations")
type UnattachedEIPRule struct{BaseRule}
func NewUnattachedEIPRule()*UnattachedEIPRule{return&UnattachedEIPRule{BaseRule:BaseRule{id:"unattached-eip",name:"Unattached Elastic IPs",description:"EIPs not associated with any resource",resourceTypes:[]string{"aws_eip"}}}}
func(r*UnattachedEIPRule)Evaluate(ctx*rec.RuleContext)([]rec.Recommendation,error){q:=`SELECT e.allocation_id,e.public_ip,e.account_id,e.region FROM elastic_ips e WHERE e.association_id IS NULL OR e.association_id=''`;rows,err:=ctx.DB.Query(q);if err!=nil{return nil,err};defer rows.Close();var recs[]rec.Recommendation;for rows.Next(){var allocID,publicIP,acct,region string;if err:=rows.Scan(&allocID,&publicIP,&acct,&region);err!=nil{continue};r:=newRec(r.id,allocID,"elastic_ip","",acct,region,rec.TypeUnattachedResource,fmt.Sprintf("EIP %s (%s) not associated",publicIP,allocID),"Release the Elastic IP",3.60,rec.ConfidenceHigh);r.TerraformCode=fmt.Sprintf("# Release EIP: %s\n# aws ec2 release-address --allocation-id %s",allocID,allocID);recs=append(recs,r)};return recs,rows.Err()}
GOEOF

# rules/oversized_ec2.go
cat > backend/internal/recommendations/rules/oversized_ec2.go << 'GOEOF'
package rules
import("fmt";"strings";rec"github.com/finopsmind/backend/internal/recommendations")
type OversizedEC2Rule struct{BaseRule;CPUThreshold float64;LookbackDays int}
func NewOversizedEC2Rule()*OversizedEC2Rule{return&OversizedEC2Rule{BaseRule:BaseRule{id:"oversized-ec2",name:"Oversized EC2 Instances",description:"EC2 using <20% CPU consistently",resourceTypes:[]string{"aws_ec2_instance"}},CPUThreshold:20.0,LookbackDays:14}}
func(r*OversizedEC2Rule)Evaluate(ctx*rec.RuleContext)([]rec.Recommendation,error){q:=fmt.Sprintf(`SELECT i.instance_id,i.instance_type,i.arn,i.account_id,i.region,COALESCE(m.avg_cpu,0),COALESCE(m.max_cpu,0)FROM ec2_instances i LEFT JOIN(SELECT resource_id,AVG(value)avg_cpu,MAX(value)max_cpu FROM cloudwatch_metrics WHERE metric_name='CPUUtilization'AND timestamp>NOW()-INTERVAL'%d days'GROUP BY resource_id)m ON i.instance_id=m.resource_id WHERE i.state='running'AND COALESCE(m.avg_cpu,0)BETWEEN 5 AND $1 AND COALESCE(m.max_cpu,0)<80`,r.LookbackDays);rows,err:=ctx.DB.Query(q,r.CPUThreshold);if err!=nil{return nil,err};defer rows.Close();var recs[]rec.Recommendation;for rows.Next(){var id,itype,arn,acct,region string;var avgCPU,maxCPU float64;if err:=rows.Scan(&id,&itype,&arn,&acct,&region,&avgCPU,&maxCPU);err!=nil{continue};newType:=suggestSmallerEC2(itype,avgCPU);if newType==itype{continue};curPrice,_:=ctx.PricingData.GetEC2HourlyPrice(itype,region);newPrice,_:=ctx.PricingData.GetEC2HourlyPrice(newType,region);if curPrice==0{curPrice=0.10};if newPrice==0{newPrice=curPrice*0.5};r:=newRec(r.id,id,"ec2_instance",arn,acct,region,rec.TypeOversized,fmt.Sprintf("%s using %.1f%% CPU",itype,avgCPU),fmt.Sprintf("Rightsize to %s",newType),(curPrice-newPrice)*hoursPerMonth,rec.ConfidenceMedium);r.TerraformCode=fmt.Sprintf("# Rightsize EC2: %s -> %s\n# instance_type = \"%s\"",id,newType,newType);recs=append(recs,r)};return recs,rows.Err()}
func suggestSmallerEC2(current string,avgCPU float64)string{sizes:=[]string{"nano","micro","small","medium","large","xlarge","2xlarge","4xlarge","8xlarge"};parts:=strings.Split(current,".");if len(parts)!=2{return current};family,curSize:=parts[0],parts[1];curIdx:=-1;for i,s:=range sizes{if s==curSize{curIdx=i;break}};if curIdx<=0{return current};steps:=1;if avgCPU<10{steps=2};newIdx:=curIdx-steps;if newIdx<0{newIdx=0};return family+"."+sizes[newIdx]}
GOEOF

# rules/oversized_rds.go
cat > backend/internal/recommendations/rules/oversized_rds.go << 'GOEOF'
package rules
import("fmt";"strings";rec"github.com/finopsmind/backend/internal/recommendations")
type OversizedRDSRule struct{BaseRule;CPUThreshold float64;LookbackDays int}
func NewOversizedRDSRule()*OversizedRDSRule{return&OversizedRDSRule{BaseRule:BaseRule{id:"oversized-rds",name:"Oversized RDS Instances",description:"RDS with consistently low CPU utilization",resourceTypes:[]string{"aws_rds_instance"}},CPUThreshold:25.0,LookbackDays:14}}
func(r*OversizedRDSRule)Evaluate(ctx*rec.RuleContext)([]rec.Recommendation,error){q:=fmt.Sprintf(`SELECT d.db_instance_id,d.db_instance_class,d.engine,d.arn,d.account_id,d.region,d.multi_az,COALESCE(m.avg_cpu,0),COALESCE(m.max_cpu,0)FROM rds_instances d LEFT JOIN(SELECT resource_id,AVG(value)avg_cpu,MAX(value)max_cpu FROM cloudwatch_metrics WHERE metric_name='CPUUtilization'AND timestamp>NOW()-INTERVAL'%d days'GROUP BY resource_id)m ON d.db_instance_id=m.resource_id WHERE d.status='available'AND COALESCE(m.avg_cpu,0)BETWEEN 5 AND $1 AND COALESCE(m.max_cpu,0)<70`,r.LookbackDays);rows,err:=ctx.DB.Query(q,r.CPUThreshold);if err!=nil{return nil,err};defer rows.Close();var recs[]rec.Recommendation;for rows.Next(){var id,class,engine,arn,acct,region string;var multiAZ bool;var avgCPU,maxCPU float64;if err:=rows.Scan(&id,&class,&engine,&arn,&acct,&region,&multiAZ,&avgCPU,&maxCPU);err!=nil{continue};newClass:=suggestSmallerRDS(class,avgCPU);if newClass==class{continue};curPrice,_:=ctx.PricingData.GetRDSHourlyPrice(class,engine,region,multiAZ);newPrice,_:=ctx.PricingData.GetRDSHourlyPrice(newClass,engine,region,multiAZ);if curPrice==0{curPrice=0.20};if newPrice==0{newPrice=curPrice*0.5};r:=newRec(r.id,id,"rds_instance",arn,acct,region,rec.TypeOversized,fmt.Sprintf("%s (%s) using %.1f%% CPU",class,engine,avgCPU),fmt.Sprintf("Rightsize to %s",newClass),(curPrice-newPrice)*hoursPerMonth,rec.ConfidenceMedium);recs=append(recs,r)};return recs,rows.Err()}
func suggestSmallerRDS(current string,avgCPU float64)string{sizes:=[]string{"micro","small","medium","large","xlarge","2xlarge","4xlarge","8xlarge"};parts:=strings.Split(strings.TrimPrefix(current,"db."),".");if len(parts)!=2{return current};family,curSize:=parts[0],parts[1];curIdx:=-1;for i,s:=range sizes{if s==curSize{curIdx=i;break}};if curIdx<=0{return current};steps:=1;if avgCPU<15{steps=2};newIdx:=curIdx-steps;if newIdx<0{newIdx=0};return"db."+family+"."+sizes[newIdx]}
GOEOF

# rules/oversized_lambda.go
cat > backend/internal/recommendations/rules/oversized_lambda.go << 'GOEOF'
package rules
import("fmt";rec"github.com/finopsmind/backend/internal/recommendations")
type OversizedLambdaRule struct{BaseRule;MemoryUsageThreshold float64;LookbackDays int}
func NewOversizedLambdaRule()*OversizedLambdaRule{return&OversizedLambdaRule{BaseRule:BaseRule{id:"oversized-lambda",name:"Oversized Lambda Functions",description:"Lambda using <50% of allocated memory",resourceTypes:[]string{"aws_lambda_function"}},MemoryUsageThreshold:50.0,LookbackDays:14}}
func(r*OversizedLambdaRule)Evaluate(ctx*rec.RuleContext)([]rec.Recommendation,error){q:=fmt.Sprintf(`SELECT l.function_name,l.arn,l.account_id,l.region,l.memory_size,COALESCE(m.max_mem,0),COALESCE(m.invocations,0),COALESCE(m.avg_dur,0)FROM lambda_functions l LEFT JOIN(SELECT resource_id,MAX(CASE WHEN metric_name='MemoryUtilization'THEN value END)max_mem,SUM(CASE WHEN metric_name='Invocations'THEN value END)invocations,AVG(CASE WHEN metric_name='Duration'THEN value END)avg_dur FROM cloudwatch_metrics WHERE metric_name IN('MemoryUtilization','Invocations','Duration')AND timestamp>NOW()-INTERVAL'%d days'GROUP BY resource_id)m ON l.function_name=m.resource_id WHERE COALESCE(m.invocations,0)>100 AND COALESCE(m.max_mem,0)<$1 AND l.memory_size>128`,r.LookbackDays);rows,err:=ctx.DB.Query(q,r.MemoryUsageThreshold);if err!=nil{return nil,err};defer rows.Close();var recs[]rec.Recommendation;for rows.Next(){var name,arn,acct,region string;var memSize int;var maxMem,invocations,avgDur float64;if err:=rows.Scan(&name,&arn,&acct,&region,&memSize,&maxMem,&invocations,&avgDur);err!=nil{continue};recMB:=int((maxMem*float64(memSize)/100.0+63)/64)*64;if recMB<128{recMB=128};if recMB>=memSize{continue};curCost:=(float64(memSize)/1024.0)*(avgDur/1000.0)*invocations*0.0000166667*30;newCost:=(float64(recMB)/1024.0)*(avgDur/1000.0)*invocations*0.0000166667*30;savings:=curCost-newCost;if savings<1{continue};r:=newRec(r.id,name,"lambda_function",arn,acct,region,rec.TypeOversized,fmt.Sprintf("Lambda %dMB allocated, %.1f%% used",memSize,maxMem),fmt.Sprintf("Reduce memory to %dMB",recMB),savings,rec.ConfidenceMedium);recs=append(recs,r)};return recs,rows.Err()}
GOEOF

# rules/missing_s3_lifecycle.go
cat > backend/internal/recommendations/rules/missing_s3_lifecycle.go << 'GOEOF'
package rules
import("fmt";rec"github.com/finopsmind/backend/internal/recommendations")
type MissingS3LifecycleRule struct{BaseRule;MinBucketSizeGB float64}
func NewMissingS3LifecycleRule()*MissingS3LifecycleRule{return&MissingS3LifecycleRule{BaseRule:BaseRule{id:"missing-s3-lifecycle",name:"S3 Buckets Without Lifecycle Policies",description:"Large S3 buckets (>100GB) without lifecycle rules",resourceTypes:[]string{"aws_s3_bucket"}},MinBucketSizeGB:100.0}}
func(r*MissingS3LifecycleRule)Evaluate(ctx*rec.RuleContext)([]rec.Recommendation,error){q:=`SELECT b.bucket_name,b.arn,b.account_id,b.region,b.size_bytes,b.object_count FROM s3_buckets b WHERE b.has_lifecycle_rules=false AND b.size_bytes>$1`;rows,err:=ctx.DB.Query(q,int64(r.MinBucketSizeGB*1024*1024*1024));if err!=nil{return nil,err};defer rows.Close();var recs[]rec.Recommendation;for rows.Next(){var name,arn,acct,region string;var sizeBytes int64;var objCount int;if err:=rows.Scan(&name,&arn,&acct,&region,&sizeBytes,&objCount);err!=nil{continue};sizeGB:=float64(sizeBytes)/(1024*1024*1024);savings:=sizeGB*0.3*(0.023-0.0125);r:=newRec(r.id,name,"s3_bucket",arn,acct,region,rec.TypeMissingOptimization,fmt.Sprintf("Bucket '%s' has %.1fGB without lifecycle rules",name,sizeGB),"Add lifecycle policy for automatic tiering",savings,rec.ConfidenceMedium);r.TerraformCode=fmt.Sprintf("# Add lifecycle to S3: %s\nresource \"aws_s3_bucket_lifecycle_configuration\" \"lifecycle\" {\n  bucket = \"%s\"\n  rule {\n    id = \"transition-to-ia\"\n    status = \"Enabled\"\n    transition { days = 30; storage_class = \"STANDARD_IA\" }\n  }\n}",name,name);recs=append(recs,r)};return recs,rows.Err()}
GOEOF

# rules/missing_s3_tiering.go
cat > backend/internal/recommendations/rules/missing_s3_tiering.go << 'GOEOF'
package rules
import("fmt";rec"github.com/finopsmind/backend/internal/recommendations")
type MissingS3TieringRule struct{BaseRule;MinBucketSizeGB float64}
func NewMissingS3TieringRule()*MissingS3TieringRule{return&MissingS3TieringRule{BaseRule:BaseRule{id:"missing-s3-tiering",name:"S3 Without Intelligent-Tiering",description:"Buckets that could use Intelligent-Tiering",resourceTypes:[]string{"aws_s3_bucket"}},MinBucketSizeGB:50.0}}
func(r*MissingS3TieringRule)Evaluate(ctx*rec.RuleContext)([]rec.Recommendation,error){q:=`SELECT b.bucket_name,b.arn,b.account_id,b.region,b.size_bytes FROM s3_buckets b WHERE b.primary_storage_class='STANDARD'AND b.size_bytes>$1`;rows,err:=ctx.DB.Query(q,int64(r.MinBucketSizeGB*1024*1024*1024));if err!=nil{return nil,err};defer rows.Close();var recs[]rec.Recommendation;for rows.Next(){var name,arn,acct,region string;var sizeBytes int64;if err:=rows.Scan(&name,&arn,&acct,&region,&sizeBytes);err!=nil{continue};sizeGB:=float64(sizeBytes)/(1024*1024*1024);savings:=sizeGB*0.023*0.20;r:=newRec(r.id,name,"s3_bucket",arn,acct,region,rec.TypeMissingOptimization,fmt.Sprintf("Bucket '%s' (%.1fGB) without Intelligent-Tiering",name,sizeGB),"Enable Intelligent-Tiering",savings,rec.ConfidenceLow);recs=append(recs,r)};return recs,rows.Err()}
GOEOF

# rules/gp2_to_gp3.go
cat > backend/internal/recommendations/rules/gp2_to_gp3.go << 'GOEOF'
package rules
import("fmt";rec"github.com/finopsmind/backend/internal/recommendations")
type GP2ToGP3Rule struct{BaseRule}
func NewGP2ToGP3Rule()*GP2ToGP3Rule{return&GP2ToGP3Rule{BaseRule:BaseRule{id:"gp2-to-gp3",name:"Upgrade GP2 to GP3 EBS Volumes",description:"gp2 volumes that save 20% by upgrading to gp3",resourceTypes:[]string{"aws_ebs_volume"}}}}
func(r*GP2ToGP3Rule)Evaluate(ctx*rec.RuleContext)([]rec.Recommendation,error){q:=`SELECT v.volume_id,v.arn,v.account_id,v.region,v.size_gb FROM ebs_volumes v WHERE v.volume_type='gp2'AND v.state IN('available','in-use')`;rows,err:=ctx.DB.Query(q);if err!=nil{return nil,err};defer rows.Close();var recs[]rec.Recommendation;for rows.Next(){var id,arn,acct,region string;var sizeGB int;if err:=rows.Scan(&id,&arn,&acct,&region,&sizeGB);err!=nil{continue};savings:=float64(sizeGB)*0.10-float64(sizeGB)*0.08;if savings<=0{continue};r:=newRec(r.id,id,"ebs_volume",arn,acct,region,rec.TypeMissingOptimization,fmt.Sprintf("gp2 volume %s (%dGB)",id,sizeGB),"Upgrade to gp3 for 20% savings",savings,rec.ConfidenceHigh);r.TerraformCode=fmt.Sprintf("# Upgrade to gp3: %s\n# aws ec2 modify-volume --volume-id %s --volume-type gp3 --iops 3000 --throughput 125",id,id);recs=append(recs,r)};return recs,rows.Err()}
GOEOF

# rules/nat_gateway_endpoints.go
cat > backend/internal/recommendations/rules/nat_gateway_endpoints.go << 'GOEOF'
package rules
import("fmt";rec"github.com/finopsmind/backend/internal/recommendations")
type NATGatewayEndpointsRule struct{BaseRule;MinMonthlyGBProcessed float64}
func NewNATGatewayEndpointsRule()*NATGatewayEndpointsRule{return&NATGatewayEndpointsRule{BaseRule:BaseRule{id:"nat-gateway-endpoints",name:"NAT Gateway VPC Endpoint Opportunity",description:"NAT Gateways with high S3/DynamoDB traffic",resourceTypes:[]string{"aws_nat_gateway"}},MinMonthlyGBProcessed:100.0}}
func(r*NATGatewayEndpointsRule)Evaluate(ctx*rec.RuleContext)([]rec.Recommendation,error){q:=`SELECT n.nat_gateway_id,n.vpc_id,n.account_id,n.region,COALESCE(m.s3_gb,0),COALESCE(m.dynamodb_gb,0)FROM nat_gateways n LEFT JOIN(SELECT resource_id,SUM(CASE WHEN destination_service='s3'THEN value END)/(1024*1024*1024)s3_gb,SUM(CASE WHEN destination_service='dynamodb'THEN value END)/(1024*1024*1024)dynamodb_gb FROM vpc_flow_logs WHERE timestamp>NOW()-INTERVAL'30 days'GROUP BY resource_id)m ON n.nat_gateway_id=m.resource_id WHERE n.state='available'AND(COALESCE(m.s3_gb,0)+COALESCE(m.dynamodb_gb,0))>$1`;rows,err:=ctx.DB.Query(q,r.MinMonthlyGBProcessed);if err!=nil{return nil,err};defer rows.Close();var recs[]rec.Recommendation;for rows.Next(){var natID,vpcID,acct,region string;var s3GB,dynamoDBGB float64;if err:=rows.Scan(&natID,&vpcID,&acct,&region,&s3GB,&dynamoDBGB);err!=nil{continue};savings:=(s3GB+dynamoDBGB)*0.045;r:=newRec(r.id,natID,"nat_gateway","",acct,region,rec.TypeNetworkingWaste,fmt.Sprintf("NAT processing %.1fGB S3 + %.1fGB DynamoDB",s3GB,dynamoDBGB),"Add VPC Gateway Endpoints for S3 and DynamoDB",savings,rec.ConfidenceHigh);r.TerraformCode=fmt.Sprintf("# Add VPC endpoints for %s\nresource \"aws_vpc_endpoint\" \"s3\" {\n  vpc_id = \"%s\"\n  service_name = \"com.amazonaws.%s.s3\"\n  vpc_endpoint_type = \"Gateway\"\n}",vpcID,vpcID,region);recs=append(recs,r)};return recs,rows.Err()}
GOEOF

# rules/old_snapshots.go
cat > backend/internal/recommendations/rules/old_snapshots.go << 'GOEOF'
package rules
import("fmt";"time";rec"github.com/finopsmind/backend/internal/recommendations")
type OldSnapshotsRule struct{BaseRule;RetentionDays int}
func NewOldSnapshotsRule()*OldSnapshotsRule{return&OldSnapshotsRule{BaseRule:BaseRule{id:"old-snapshots",name:"Old EBS Snapshots",description:"EBS snapshots older than 90 days",resourceTypes:[]string{"aws_ebs_snapshot"}},RetentionDays:90}}
func(r*OldSnapshotsRule)Evaluate(ctx*rec.RuleContext)([]rec.Recommendation,error){q:=fmt.Sprintf(`SELECT s.snapshot_id,s.volume_id,s.arn,s.account_id,s.region,s.volume_size_gb,s.start_time,v.state FROM ebs_snapshots s LEFT JOIN ebs_volumes v ON s.volume_id=v.volume_id WHERE s.state='completed'AND s.start_time<NOW()-INTERVAL'%d days'ORDER BY s.volume_size_gb DESC`,r.RetentionDays);rows,err:=ctx.DB.Query(q);if err!=nil{return nil,err};defer rows.Close();var recs[]rec.Recommendation;for rows.Next(){var snapID,volID,arn,acct,region string;var sizeGB int;var startTime time.Time;var volState*string;if err:=rows.Scan(&snapID,&volID,&arn,&acct,&region,&sizeGB,&startTime,&volState);err!=nil{continue};ageInDays:=int(time.Since(startTime).Hours()/24);cost:=float64(sizeGB)*0.05;volStatus:="deleted";if volState!=nil{volStatus=*volState};conf:=rec.ConfidenceMedium;if volStatus=="deleted"{conf=rec.ConfidenceHigh};r:=newRec(r.id,snapID,"ebs_snapshot",arn,acct,region,rec.TypeRetentionCleanup,fmt.Sprintf("Snapshot %dGB, %d days old, volume %s",sizeGB,ageInDays,volStatus),"Delete snapshot if not needed",cost,conf);r.TerraformCode=fmt.Sprintf("# Delete old snapshot: %s\n# aws ec2 delete-snapshot --snapshot-id %s",snapID,snapID);recs=append(recs,r)};return recs,rows.Err()}
GOEOF

# rules/cloudwatch_retention.go
cat > backend/internal/recommendations/rules/cloudwatch_retention.go << 'GOEOF'
package rules
import("fmt";rec"github.com/finopsmind/backend/internal/recommendations")
type CloudWatchRetentionRule struct{BaseRule;MinLogSizeGB float64}
func NewCloudWatchRetentionRule()*CloudWatchRetentionRule{return&CloudWatchRetentionRule{BaseRule:BaseRule{id:"cloudwatch-retention",name:"CloudWatch Logs Without Retention",description:"Log groups without retention policies",resourceTypes:[]string{"aws_cloudwatch_log_group"}},MinLogSizeGB:1.0}}
func(r*CloudWatchRetentionRule)Evaluate(ctx*rec.RuleContext)([]rec.Recommendation,error){q:=`SELECT l.log_group_name,l.arn,l.account_id,l.region,l.stored_bytes FROM cloudwatch_log_groups l WHERE(l.retention_days IS NULL OR l.retention_days=0)AND l.stored_bytes>$1`;rows,err:=ctx.DB.Query(q,int64(r.MinLogSizeGB*1024*1024*1024));if err!=nil{return nil,err};defer rows.Close();var recs[]rec.Recommendation;for rows.Next(){var name,arn,acct,region string;var storedBytes int64;if err:=rows.Scan(&name,&arn,&acct,&region,&storedBytes);err!=nil{continue};storedGB:=float64(storedBytes)/(1024*1024*1024);savings:=storedGB*0.03*0.9;r:=newRec(r.id,name,"cloudwatch_log_group",arn,acct,region,rec.TypeRetentionCleanup,fmt.Sprintf("Log group '%s' has %.1fGB with no retention",name,storedGB),"Set retention policy (30-90 days recommended)",savings,rec.ConfidenceHigh);r.TerraformCode=fmt.Sprintf("# Set retention: %s\nresource \"aws_cloudwatch_log_group\" \"with_retention\" {\n  name = \"%s\"\n  retention_in_days = 30\n}",name,name);recs=append(recs,r)};return recs,rows.Err()}
GOEOF

# rules/unused_ecr.go
cat > backend/internal/recommendations/rules/unused_ecr.go << 'GOEOF'
package rules
import("fmt";"time";rec"github.com/finopsmind/backend/internal/recommendations")
type UnusedECRRule struct{BaseRule;DaysSinceLastPull int}
func NewUnusedECRRule()*UnusedECRRule{return&UnusedECRRule{BaseRule:BaseRule{id:"unused-ecr",name:"Unused ECR Images",description:"ECR images not pulled in 90+ days",resourceTypes:[]string{"aws_ecr_repository"}},DaysSinceLastPull:90}}
func(r*UnusedECRRule)Evaluate(ctx*rec.RuleContext)([]rec.Recommendation,error){q:=fmt.Sprintf(`SELECT e.repository_name,e.arn,e.account_id,e.region,e.image_count,e.total_size_bytes,e.last_image_pull FROM ecr_repositories e WHERE e.last_image_pull<NOW()-INTERVAL'%d days'OR e.last_image_pull IS NULL`,r.DaysSinceLastPull);rows,err:=ctx.DB.Query(q);if err!=nil{return nil,err};defer rows.Close();var recs[]rec.Recommendation;for rows.Next(){var name,arn,acct,region string;var imgCount int;var sizeBytes int64;var lastPull*time.Time;if err:=rows.Scan(&name,&arn,&acct,&region,&imgCount,&sizeBytes,&lastPull);err!=nil{continue};sizeGB:=float64(sizeBytes)/(1024*1024*1024);cost:=sizeGB*0.10;if cost<1{continue};lastPullInfo:="never";if lastPull!=nil{lastPullInfo=fmt.Sprintf("%d days ago",int(time.Since(*lastPull).Hours()/24))};r:=newRec(r.id,name,"ecr_repository",arn,acct,region,rec.TypeRetentionCleanup,fmt.Sprintf("ECR '%s' (%d images, %.1fGB) last pulled %s",name,imgCount,sizeGB,lastPullInfo),"Add lifecycle policy or delete unused repo",cost,rec.ConfidenceMedium);recs=append(recs,r)};return recs,rows.Err()}
GOEOF

# rules/multi_az_dev.go
cat > backend/internal/recommendations/rules/multi_az_dev.go << 'GOEOF'
package rules
import("fmt";rec"github.com/finopsmind/backend/internal/recommendations")
type MultiAZDevRule struct{BaseRule;DevEnvTags[]string}
func NewMultiAZDevRule()*MultiAZDevRule{return&MultiAZDevRule{BaseRule:BaseRule{id:"multi-az-dev",name:"Multi-AZ on Dev/Test RDS",description:"Dev/test RDS with Multi-AZ enabled",resourceTypes:[]string{"aws_rds_instance"}},DevEnvTags:[]string{"dev","development","test","staging","qa","sandbox"}}}
func(r*MultiAZDevRule)Evaluate(ctx*rec.RuleContext)([]rec.Recommendation,error){q:=`SELECT d.db_instance_id,d.db_instance_class,d.engine,d.arn,d.account_id,d.region,d.tags->>'Environment'FROM rds_instances d WHERE d.multi_az=true AND d.status='available'AND(LOWER(d.tags->>'Environment')SIMILAR TO'%(dev|test|staging|qa|sandbox)%'OR LOWER(d.tags->>'Name')SIMILAR TO'%(dev|test|staging|qa|sandbox)%'OR LOWER(d.db_instance_id)SIMILAR TO'%(dev|test|staging|qa|sandbox)%')`;rows,err:=ctx.DB.Query(q);if err!=nil{return nil,err};defer rows.Close();var recs[]rec.Recommendation;for rows.Next(){var id,class,engine,arn,acct,region string;var envTag*string;if err:=rows.Scan(&id,&class,&engine,&arn,&acct,&region,&envTag);err!=nil{continue};singleAZPrice,_:=ctx.PricingData.GetRDSHourlyPrice(class,engine,region,false);multiAZPrice,_:=ctx.PricingData.GetRDSHourlyPrice(class,engine,region,true);if singleAZPrice==0{singleAZPrice=0.10};if multiAZPrice==0{multiAZPrice=singleAZPrice*2};savings:=(multiAZPrice-singleAZPrice)*hoursPerMonth;env:="dev/test";if envTag!=nil{env=*envTag};r:=newRec(r.id,id,"rds_instance",arn,acct,region,rec.TypeMissingOptimization,fmt.Sprintf("Dev/test RDS '%s' (%s, %s) has Multi-AZ",id,class,env),"Disable Multi-AZ for non-production",savings,rec.ConfidenceHigh);r.TerraformCode=fmt.Sprintf("# Disable Multi-AZ: %s\n# aws rds modify-db-instance --db-instance-identifier %s --no-multi-az",id,id);recs=append(recs,r)};return recs,rows.Err()}
GOEOF

# rules/idle_elasticache.go
cat > backend/internal/recommendations/rules/idle_elasticache.go << 'GOEOF'
package rules
import("fmt";rec"github.com/finopsmind/backend/internal/recommendations")
type IdleElastiCacheRule struct{BaseRule;HitRateThreshold float64;LookbackDays int}
func NewIdleElastiCacheRule()*IdleElastiCacheRule{return&IdleElastiCacheRule{BaseRule:BaseRule{id:"idle-elasticache",name:"Idle ElastiCache Clusters",description:"ElastiCache with <1% cache hit rate",resourceTypes:[]string{"aws_elasticache_cluster"}},HitRateThreshold:1.0,LookbackDays:14}}
func(r*IdleElastiCacheRule)Evaluate(ctx*rec.RuleContext)([]rec.Recommendation,error){q:=fmt.Sprintf(`SELECT c.cluster_id,c.arn,c.account_id,c.region,c.engine,c.node_type,c.num_nodes,COALESCE(m.cache_hits,0),COALESCE(m.cache_misses,0),COALESCE(m.curr_conn,0)FROM elasticache_clusters c LEFT JOIN(SELECT resource_id,SUM(CASE WHEN metric_name='CacheHits'THEN value END)cache_hits,SUM(CASE WHEN metric_name='CacheMisses'THEN value END)cache_misses,AVG(CASE WHEN metric_name='CurrConnections'THEN value END)curr_conn FROM cloudwatch_metrics WHERE metric_name IN('CacheHits','CacheMisses','CurrConnections')AND timestamp>NOW()-INTERVAL'%d days'GROUP BY resource_id)m ON c.cluster_id=m.resource_id WHERE c.status='available'`,r.LookbackDays);rows,err:=ctx.DB.Query(q);if err!=nil{return nil,err};defer rows.Close();var recs[]rec.Recommendation;for rows.Next(){var clusterID,arn,acct,region,engine,nodeType string;var numNodes int;var cacheHits,cacheMisses,currConn float64;if err:=rows.Scan(&clusterID,&arn,&acct,&region,&engine,&nodeType,&numNodes,&cacheHits,&cacheMisses,&currConn);err!=nil{continue};totalReq:=cacheHits+cacheMisses;hitRate:=0.0;if totalReq>0{hitRate=(cacheHits/totalReq)*100};if hitRate>r.HitRateThreshold||(totalReq==0&&currConn>1){continue};cost:=0.10*hoursPerMonth*float64(numNodes);r:=newRec(r.id,clusterID,"elasticache_cluster",arn,acct,region,rec.TypeIdleResource,fmt.Sprintf("ElastiCache '%s' (%s, %d nodes) %.1f%% hit rate",clusterID,nodeType,numNodes,hitRate),"Delete unused cache cluster",cost,rec.ConfidenceMedium);recs=append(recs,r)};return recs,rows.Err()}
GOEOF

# rules/dynamodb_provisioned.go
cat > backend/internal/recommendations/rules/dynamodb_provisioned.go << 'GOEOF'
package rules
import("fmt";rec"github.com/finopsmind/backend/internal/recommendations")
type DynamoDBProvisionedRule struct{BaseRule;UtilizationThreshold float64;LookbackDays int}
func NewDynamoDBProvisionedRule()*DynamoDBProvisionedRule{return&DynamoDBProvisionedRule{BaseRule:BaseRule{id:"dynamodb-provisioned",name:"DynamoDB Provisioned to On-Demand",description:"DynamoDB with <25% capacity utilization",resourceTypes:[]string{"aws_dynamodb_table"}},UtilizationThreshold:25.0,LookbackDays:14}}
func(r*DynamoDBProvisionedRule)Evaluate(ctx*rec.RuleContext)([]rec.Recommendation,error){q:=fmt.Sprintf(`SELECT t.table_name,t.arn,t.account_id,t.region,t.read_capacity_units,t.write_capacity_units,COALESCE(m.consumed_rcu,0),COALESCE(m.consumed_wcu,0)FROM dynamodb_tables t LEFT JOIN(SELECT resource_id,AVG(CASE WHEN metric_name='ConsumedReadCapacityUnits'THEN value END)consumed_rcu,AVG(CASE WHEN metric_name='ConsumedWriteCapacityUnits'THEN value END)consumed_wcu FROM cloudwatch_metrics WHERE metric_name IN('ConsumedReadCapacityUnits','ConsumedWriteCapacityUnits')AND timestamp>NOW()-INTERVAL'%d days'GROUP BY resource_id)m ON t.table_name=m.resource_id WHERE t.billing_mode='PROVISIONED'AND t.read_capacity_units>0`,r.LookbackDays);rows,err:=ctx.DB.Query(q);if err!=nil{return nil,err};defer rows.Close();var recs[]rec.Recommendation;for rows.Next(){var tableName,arn,acct,region string;var readCap,writeCap int;var consumedRCU,consumedWCU float64;if err:=rows.Scan(&tableName,&arn,&acct,&region,&readCap,&writeCap,&consumedRCU,&consumedWCU);err!=nil{continue};readUtil,writeUtil:=0.0,0.0;if readCap>0{readUtil=(consumedRCU/float64(readCap))*100};if writeCap>0{writeUtil=(consumedWCU/float64(writeCap))*100};avgUtil:=(readUtil+writeUtil)/2;if avgUtil>r.UtilizationThreshold{continue};provCost:=(float64(readCap)*0.00065+float64(writeCap)*0.00065)*hoursPerMonth;onDemandCost:=(consumedRCU*hoursPerMonth/1000000)*0.25+(consumedWCU*hoursPerMonth/1000000)*1.25;savings:=provCost-onDemandCost;if savings<10{continue};r:=newRec(r.id,tableName,"dynamodb_table",arn,acct,region,rec.TypeMissingOptimization,fmt.Sprintf("DynamoDB '%s' provisioned %d/%d RCU/WCU, using %.1f%%",tableName,readCap,writeCap,avgUtil),"Switch to on-demand billing",savings,rec.ConfidenceMedium);r.TerraformCode=fmt.Sprintf("# Switch to on-demand: %s\n# aws dynamodb update-table --table-name %s --billing-mode PAY_PER_REQUEST",tableName,tableName);recs=append(recs,r)};return recs,rows.Err()}
GOEOF

# rules/register.go
cat > backend/internal/recommendations/rules/register.go << 'GOEOF'
package rules
import rec"github.com/finopsmind/backend/internal/recommendations"
func RegisterAllRules(engine*rec.Engine){engine.RegisterRules(NewIdleEC2Rule(),NewIdleRDSRule(),NewIdleELBRule(),NewIdleElastiCacheRule(),NewUnattachedEBSRule(),NewUnattachedEIPRule(),NewOversizedEC2Rule(),NewOversizedRDSRule(),NewOversizedLambdaRule(),NewMissingS3LifecycleRule(),NewMissingS3TieringRule(),NewGP2ToGP3Rule(),NewMultiAZDevRule(),NewDynamoDBProvisionedRule(),NewNATGatewayEndpointsRule(),NewOldSnapshotsRule(),NewCloudWatchRetentionRule(),NewUnusedECRRule())}
func GetAllRuleIDs()[]string{return[]string{"idle-ec2","idle-rds","idle-elb","idle-elasticache","unattached-ebs","unattached-eip","oversized-ec2","oversized-rds","oversized-lambda","missing-s3-lifecycle","missing-s3-tiering","gp2-to-gp3","multi-az-dev","dynamodb-provisioned","nat-gateway-endpoints","old-snapshots","cloudwatch-retention","unused-ecr"}}
func GetRulesByCategory()map[rec.RecommendationType][]string{return map[rec.RecommendationType][]string{rec.TypeIdleResource:{"idle-ec2","idle-rds","idle-elb","idle-elasticache"},rec.TypeUnattachedResource:{"unattached-ebs","unattached-eip"},rec.TypeOversized:{"oversized-ec2","oversized-rds","oversized-lambda"},rec.TypeMissingOptimization:{"missing-s3-lifecycle","missing-s3-tiering","gp2-to-gp3","multi-az-dev","dynamodb-provisioned"},rec.TypeNetworkingWaste:{"nat-gateway-endpoints"},rec.TypeRetentionCleanup:{"old-snapshots","cloudwatch-retention","unused-ecr"}}}
GOEOF

# handler/recommendations.go
echo "[5/6] Creating handler..."
cat > backend/internal/handler/recommendations.go << 'GOEOF'
package handler
import("context";"encoding/json";"net/http";"strconv";"time";rec"github.com/finopsmind/backend/internal/recommendations";"github.com/finopsmind/backend/internal/recommendations/rules";"github.com/go-chi/chi/v5")
type RecommendationHandler struct{engine*rec.Engine;db rec.DBQuerier}
func NewRecommendationHandler(engine*rec.Engine,db rec.DBQuerier)*RecommendationHandler{return&RecommendationHandler{engine:engine,db:db}}
func(h*RecommendationHandler)RegisterRoutes(r chi.Router){r.Route("/recommendations",func(r chi.Router){r.Get("/",h.ListRecommendations);r.Post("/generate",h.GenerateRecommendations);r.Get("/rules",h.ListRules);r.Get("/summary",h.GetSummary);r.Get("/{id}",h.GetRecommendation);r.Put("/{id}/status",h.UpdateStatus);r.Post("/{id}/dismiss",h.DismissRecommendation);r.Get("/{id}/terraform",h.GetTerraform)})}
type GenerateReq struct{AccountIDs[]string`json:"account_ids,omitempty"`;Regions[]string`json:"regions,omitempty"`;RuleIDs[]string`json:"rule_ids,omitempty"`}
func(h*RecommendationHandler)GenerateRecommendations(w http.ResponseWriter,r*http.Request){var req GenerateReq;json.NewDecoder(r.Body).Decode(&req);ctx,cancel:=context.WithTimeout(r.Context(),5*time.Minute);defer cancel();var opts[]rec.RunOption;if len(req.AccountIDs)>0{opts=append(opts,rec.WithAccounts(req.AccountIDs...))};if len(req.Regions)>0{opts=append(opts,rec.WithRegions(req.Regions...))};if len(req.RuleIDs)>0{opts=append(opts,rec.WithRules(req.RuleIDs...))};result,err:=h.engine.RunAllRules(ctx,opts...);if err!=nil{respondError(w,500,err.Error());return};var allRecs[]rec.Recommendation;for _,rr:=range result.Results{allRecs=append(allRecs,rr.Recommendations...)};h.engine.SaveRecommendations(ctx,allRecs);respondJSON(w,200,result)}
func(h*RecommendationHandler)ListRecommendations(w http.ResponseWriter,r*http.Request){recType:=r.URL.Query().Get("type");severity:=r.URL.Query().Get("severity");status:=r.URL.Query().Get("status");page,_:=strconv.Atoi(r.URL.Query().Get("page"));if page<1{page=1};limit,_:=strconv.Atoi(r.URL.Query().Get("limit"));if limit<1||limit>100{limit=20};offset:=(page-1)*limit;q:=`SELECT id,type,rule_id,resource_id,resource_type,resource_arn,account_id,region,current_state,recommended_action,estimated_savings,confidence,severity,terraform_code,resource_metadata,status,created_at,updated_at FROM recommendations WHERE 1=1`;var args[]interface{};argCount:=0;if recType!=""{argCount++;q+=" AND type = $"+strconv.Itoa(argCount);args=append(args,recType)};if severity!=""{argCount++;q+=" AND severity = $"+strconv.Itoa(argCount);args=append(args,severity)};if status!=""{argCount++;q+=" AND status = $"+strconv.Itoa(argCount);args=append(args,status)};q+=" ORDER BY estimated_savings DESC LIMIT $"+strconv.Itoa(argCount+1)+" OFFSET $"+strconv.Itoa(argCount+2);args=append(args,limit,offset);rows,err:=h.db.Query(q,args...);if err!=nil{respondError(w,500,"Query failed");return};defer rows.Close();var recs[]rec.Recommendation;for rows.Next(){var r rec.Recommendation;rows.Scan(&r.ID,&r.Type,&r.RuleID,&r.ResourceID,&r.ResourceType,&r.ResourceARN,&r.AccountID,&r.Region,&r.CurrentState,&r.RecommendedAction,&r.EstimatedSavings,&r.Confidence,&r.Severity,&r.TerraformCode,&r.ResourceMetadata,&r.Status,&r.CreatedAt,&r.UpdatedAt);recs=append(recs,r)};respondJSON(w,200,map[string]interface{}{"recommendations":recs,"pagination":map[string]int{"page":page,"limit":limit}})}
func(h*RecommendationHandler)ListRules(w http.ResponseWriter,r*http.Request){engineRules:=h.engine.GetRules();type RuleInfo struct{ID string`json:"id"`;Name string`json:"name"`;Description string`json:"description"`;ResourceTypes[]string`json:"resource_types"`};ruleInfos:=make([]RuleInfo,0,len(engineRules));for _,rule:=range engineRules{ruleInfos=append(ruleInfos,RuleInfo{ID:rule.ID(),Name:rule.Name(),Description:rule.Description(),ResourceTypes:rule.ResourceTypes()})};respondJSON(w,200,map[string]interface{}{"rules":ruleInfos,"total":len(ruleInfos),"categories":rules.GetRulesByCategory()})}
func(h*RecommendationHandler)GetSummary(w http.ResponseWriter,r*http.Request){q:=`SELECT type,severity,status,COUNT(*),SUM(estimated_savings)FROM recommendations GROUP BY type,severity,status`;rows,err:=h.db.Query(q);if err!=nil{respondError(w,500,"Query failed");return};defer rows.Close();byType:=make(map[string]map[string]interface{});var totalSavings float64;var totalCount int;for rows.Next(){var recType,severity,status string;var count int;var savings float64;rows.Scan(&recType,&severity,&status,&count,&savings);if _,ok:=byType[recType];!ok{byType[recType]=map[string]interface{}{"count":0,"savings":0.0}};byType[recType]["count"]=byType[recType]["count"].(int)+count;byType[recType]["savings"]=byType[recType]["savings"].(float64)+savings;totalSavings+=savings;totalCount+=count};respondJSON(w,200,map[string]interface{}{"total_recommendations":totalCount,"total_estimated_savings":totalSavings,"by_type":byType})}
func(h*RecommendationHandler)GetRecommendation(w http.ResponseWriter,r*http.Request){id:=chi.URLParam(r,"id");q:=`SELECT id,type,rule_id,resource_id,resource_type,resource_arn,account_id,region,current_state,recommended_action,estimated_savings,confidence,severity,terraform_code,resource_metadata,status,created_at,updated_at FROM recommendations WHERE id=$1`;var recommendation rec.Recommendation;row:=h.db.QueryRow(q,id);if err:=row.Scan(&recommendation.ID,&recommendation.Type,&recommendation.RuleID,&recommendation.ResourceID,&recommendation.ResourceType,&recommendation.ResourceARN,&recommendation.AccountID,&recommendation.Region,&recommendation.CurrentState,&recommendation.RecommendedAction,&recommendation.EstimatedSavings,&recommendation.Confidence,&recommendation.Severity,&recommendation.TerraformCode,&recommendation.ResourceMetadata,&recommendation.Status,&recommendation.CreatedAt,&recommendation.UpdatedAt);err!=nil{respondError(w,404,"Not found");return};respondJSON(w,200,recommendation)}
func(h*RecommendationHandler)UpdateStatus(w http.ResponseWriter,r*http.Request){id:=chi.URLParam(r,"id");var req struct{Status string`json:"status"`};json.NewDecoder(r.Body).Decode(&req);h.db.Query("UPDATE recommendations SET status=$1,updated_at=$2 WHERE id=$3",req.Status,time.Now(),id);respondJSON(w,200,map[string]string{"status":"updated"})}
func(h*RecommendationHandler)DismissRecommendation(w http.ResponseWriter,r*http.Request){id:=chi.URLParam(r,"id");h.db.Query("UPDATE recommendations SET status='dismissed',updated_at=$1 WHERE id=$2",time.Now(),id);respondJSON(w,200,map[string]string{"status":"dismissed"})}
func(h*RecommendationHandler)GetTerraform(w http.ResponseWriter,r*http.Request){id:=chi.URLParam(r,"id");var tf string;h.db.QueryRow("SELECT terraform_code FROM recommendations WHERE id=$1",id).Scan(&tf);w.Header().Set("Content-Type","text/plain");w.Write([]byte(tf))}
func respondJSON(w http.ResponseWriter,status int,data interface{}){w.Header().Set("Content-Type","application/json");w.WriteHeader(status);json.NewEncoder(w).Encode(data)}
func respondError(w http.ResponseWriter,status int,msg string){respondJSON(w,status,map[string]string{"error":msg})}
GOEOF

# Database migration
echo "[6/6] Creating database migration..."
cat > backend/migrations/000004_recommendations_update.up.sql << 'SQLEOF'
ALTER TABLE recommendations ADD COLUMN IF NOT EXISTS rule_id VARCHAR(100);
ALTER TABLE recommendations ADD COLUMN IF NOT EXISTS confidence VARCHAR(20) DEFAULT 'medium';
ALTER TABLE recommendations ADD COLUMN IF NOT EXISTS terraform_code TEXT;
ALTER TABLE recommendations ADD COLUMN IF NOT EXISTS resource_metadata JSONB DEFAULT '{}';
ALTER TABLE recommendations ADD COLUMN IF NOT EXISTS resource_arn VARCHAR(500);
ALTER TABLE recommendations ADD COLUMN IF NOT EXISTS expires_at TIMESTAMP WITH TIME ZONE;
CREATE INDEX IF NOT EXISTS idx_recommendations_rule_id ON recommendations(rule_id);
CREATE INDEX IF NOT EXISTS idx_recommendations_type_severity ON recommendations(type, severity);
CREATE INDEX IF NOT EXISTS idx_recommendations_status_savings ON recommendations(status, estimated_savings DESC);
CREATE TABLE IF NOT EXISTS recommendation_history (id SERIAL PRIMARY KEY, recommendation_id VARCHAR(200) NOT NULL, action VARCHAR(50) NOT NULL, old_status VARCHAR(50), new_status VARCHAR(50), user_id VARCHAR(100), notes TEXT, created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW());
CREATE TABLE IF NOT EXISTS rule_execution_logs (id SERIAL PRIMARY KEY, run_id VARCHAR(100) NOT NULL, rule_id VARCHAR(100) NOT NULL, rule_name VARCHAR(200), started_at TIMESTAMP WITH TIME ZONE NOT NULL, completed_at TIMESTAMP WITH TIME ZONE, duration_ms INTEGER, recommendations_count INTEGER DEFAULT 0, error_message TEXT, created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW());
CREATE OR REPLACE VIEW recommendation_summary_by_account AS SELECT account_id, region, type, severity, status, COUNT(*) as count, SUM(estimated_savings) as total_savings FROM recommendations WHERE status != 'dismissed' GROUP BY account_id, region, type, severity, status;
CREATE OR REPLACE VIEW top_savings_opportunities AS SELECT id, type, rule_id, resource_id, resource_type, account_id, region, current_state, recommended_action, estimated_savings, confidence, severity, status, created_at FROM recommendations WHERE status = 'pending' ORDER BY estimated_savings DESC LIMIT 100;
SQLEOF

cat > backend/migrations/000004_recommendations_update.down.sql << 'SQLEOF'
DROP VIEW IF EXISTS top_savings_opportunities;
DROP VIEW IF EXISTS recommendation_summary_by_account;
DROP TABLE IF EXISTS rule_execution_logs;
DROP TABLE IF EXISTS recommendation_history;
DROP INDEX IF EXISTS idx_recommendations_status_savings;
DROP INDEX IF EXISTS idx_recommendations_type_severity;
DROP INDEX IF EXISTS idx_recommendations_rule_id;
ALTER TABLE recommendations DROP COLUMN IF EXISTS expires_at;
ALTER TABLE recommendations DROP COLUMN IF EXISTS resource_arn;
ALTER TABLE recommendations DROP COLUMN IF EXISTS resource_metadata;
ALTER TABLE recommendations DROP COLUMN IF EXISTS terraform_code;
ALTER TABLE recommendations DROP COLUMN IF EXISTS confidence;
ALTER TABLE recommendations DROP COLUMN IF EXISTS rule_id;
SQLEOF

echo ""
echo "============================================================================"
echo "Phase 2 Complete! Created 18 rules in backend/internal/recommendations/rules/"
echo "============================================================================"
echo ""
echo "Files created:"
echo "  - backend/internal/recommendations/types.go"
echo "  - backend/internal/recommendations/engine.go"
echo "  - backend/internal/recommendations/rules/*.go (18 rules + base + register)"
echo "  - backend/internal/handler/recommendations.go"
echo "  - backend/migrations/000004_recommendations_update.{up,down}.sql"
echo ""
echo "Next steps:"
echo "  1. Run migration: cd backend && go run cmd/migrate/main.go up"
echo "  2. Initialize engine in main.go:"
echo '     engine := rec.NewEngine(db, metricsStore, pricingData)'
echo '     rules.RegisterAllRules(engine)'
echo '     recHandler := handler.NewRecommendationHandler(engine, db)'
echo '     recHandler.RegisterRoutes(router)'
echo "  3. Test: curl -X POST http://localhost:8080/api/v1/recommendations/generate"
echo "============================================================================"
