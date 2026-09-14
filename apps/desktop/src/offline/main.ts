import * as API from './bindings/github.com/WD-Mitchell/which-model/pkg/offlinedesktop/api'
import './style.css'
import { displayRanking, type RankingResponse } from './render'

const profile=document.querySelector<HTMLSelectElement>('#profile')!
const top=document.querySelector<HTMLSelectElement>('#top')!
const error=document.querySelector<HTMLParagraphElement>('#error')!
let request=0
async function rank(){
 const id=++request;error.textContent=''
 try {
  const result=await API.Rank(profile.value,Number(top.value)) as unknown as RankingResponse
  if(id===request) displayRanking(document,result)
 } catch(e){if(id===request)error.textContent=e instanceof Error ? e.message : 'Ranking could not be completed.'}
}
async function start(){
 try{
  const profiles=(await API.Profiles()) ?? []
  if(!profiles.length) throw new Error('The bundled catalog has no profiles.')
  profiles.forEach(name=>{const o=document.createElement('option');o.value=name;o.textContent=name.replaceAll('_',' ');profile.append(o)})
  profile.value=profiles.includes('balanced_implementation')?'balanced_implementation':profiles[0]??''
  const capabilities=await API.Capabilities()
  document.querySelector('#capabilities')!.textContent=JSON.stringify(capabilities,null,2)
  await rank()
 }catch(e){error.textContent=e instanceof Error?e.message:'The offline engine could not start.'}
}
profile.addEventListener('change',rank);top.addEventListener('change',rank)
void start()
