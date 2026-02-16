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
