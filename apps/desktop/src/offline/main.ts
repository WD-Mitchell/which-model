import { Call } from '@wailsio/runtime'
import './style.css'
import { displayRanking, type RankingResponse } from './render'

const profile=document.querySelector<HTMLSelectElement>('#profile')!
const top=document.querySelector<HTMLSelectElement>('#top')!
const error=document.querySelector<HTMLParagraphElement>('#error')!
let request=0
async function rank(){
 const id=++request;error.textContent=''
 try {
  const result=await Call.ByName('offlinedesktop.API.Rank',profile.value,Number(top.value)) as RankingResponse
  if(id===request) displayRanking(document,result)
 } catch(e){if(id===request)error.textContent=e instanceof Error ? e.message : 'Ranking could not be completed.'}
}
async function start(){
 try{
  const profiles=await Call.ByName('offlinedesktop.API.Profiles') as string[]
  profiles.forEach(name=>{const o=document.createElement('option');o.value=name;o.textContent=name.replaceAll('_',' ');profile.append(o)})
  profile.value=profiles.includes('balanced_implementation')?'balanced_implementation':profiles[0]??''
  const capabilities=await Call.ByName('offlinedesktop.API.Capabilities')
  document.querySelector('#capabilities')!.textContent=JSON.stringify(capabilities,null,2)
  await rank()
 }catch(e){error.textContent=e instanceof Error?e.message:'The offline engine could not start.'}
}
profile.addEventListener('change',rank);top.addEventListener('change',rank)
void start()
