import { describe, it, expect, vi } from 'vitest'
import { Call } from '@wailsio/runtime'
import { toEngineError, createWailsHost } from './wailsHost'
import { ProfilesAPI } from '../bindings/github.com/WD-Mitchell/which-model/cmd/which-model-desktop/index.js'

describe('native structured error transport', () => {
 it('reads the actual Wails RuntimeError cause and direct DTOs', () => {
  const dto = { code: 'conflict', message: 'Profile exists' }
  expect(toEngineError(new Call.RuntimeError('wrapped', { cause: dto }))).toEqual(dto)
  expect(toEngineError(dto)).toEqual(dto)
 })
 it.each([null, [], { code: 3, message: 'bad' }, { code: 'made_up', message: 'bad' }, { code: 'conflict', message: null }])('rejects malformed cause %j', (cause) => {
  expect(toEngineError(new Error('fallback', { cause }))).toEqual({ code: 'io_error', message: 'fallback' })
 })
 it('normalizes generated call rejections', async () => {
  const dto = { code: 'not_found', message: 'Missing' }
  vi.spyOn(ProfilesAPI, 'Get').mockImplementation(() => Promise.reject(new Call.RuntimeError('wrapped', { cause: dto })) as ReturnType<typeof ProfilesAPI.Get>)
  await expect(createWailsHost().profiles.get('missing')).rejects.toEqual(dto)
  vi.restoreAllMocks()
 })
})
