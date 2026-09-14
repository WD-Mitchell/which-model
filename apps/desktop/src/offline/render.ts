interface Model {model: string; reasoning: string; total_score: string; warnings?: string[]}
export interface RankingResponse {ranking: {candidate_count: number; recommendation: Model; alternatives: Model[]}}
export function displayRanking(doc: Document,result: RankingResponse){
 const list=doc.querySelector('#results')!;list.replaceChildren()
 doc.querySelector('#summary')!.textContent=`${result.ranking.candidate_count} eligible catalog entries · score-only recommendations`
 for(const model of [result.ranking.recommendation,...result.ranking.alternatives]){
  const row=doc.createElement('li');const title=doc.createElement('h2');title.textContent=model.model
  const detail=doc.createElement('p');detail.textContent=`${model.reasoning} reasoning · score ${Number(model.total_score).toFixed(4)}`
  row.append(title,detail)
  for(const warning of model.warnings??[]){const p=doc.createElement('p');p.className='warning';p.textContent=warning;row.append(p)}
  list.append(row)
 }
}
