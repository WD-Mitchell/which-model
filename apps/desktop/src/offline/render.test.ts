import {it,expect} from 'vitest'
import {displayRanking} from './render'
it('renders catalog strings as text and shows score-only evidence',()=>{
 document.body.innerHTML='<p id="summary"></p><ol id="results"></ol>'
 displayRanking(document,{ranking:{candidate_count:1,recommendation:{model:'<img src=x onerror=alert(1)>',reasoning:'high',total_score:'0.75',warnings:['Allowance unverified']},alternatives:[]}})
 expect(document.querySelectorAll('img')).toHaveLength(0)
 expect(document.body.textContent).toContain('score-only recommendations')
 expect(document.body.textContent).toContain('Allowance unverified')
})
