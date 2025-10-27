import '@testing-library/jest-dom'

const originalError = console.error
const originalWarn = console.warn

beforeAll(() => {
    console.error = (...args: unknown[]) => {
        originalError(...args)
        throw new Error(`console.error called in tests: ${args.join(' ')}`)
    }
    console.warn = (...args: unknown[]) => {
        originalWarn(...args)
        throw new Error(`console.warn called in tests: ${args.join(' ')}`)
    }
})

afterAll(() => {
    console.error = originalError
    console.warn = originalWarn
})


