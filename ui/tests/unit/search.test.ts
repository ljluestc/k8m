import { binarySearch, linearSearch, lowerBound, upperBound } from '@/utils/algorithms/search'

describe('Search algorithms', () => {
    it('linearSearch finds elements or returns -1', () => {
        const arr = [5, 3, 9, 1]
        expect(linearSearch(arr, 5)).toBe(0)
        expect(linearSearch(arr, 1)).toBe(3)
        expect(linearSearch(arr, 7)).toBe(-1)
    })

    it('binarySearch finds indices in sorted array', () => {
        const arr = [1, 2, 3, 4, 5]
        expect(binarySearch(arr, 1)).toBe(0)
        expect(binarySearch(arr, 3)).toBe(2)
        expect(binarySearch(arr, 5)).toBe(4)
        expect(binarySearch(arr, 6)).toBe(-1)
        expect(binarySearch(arr, 0)).toBe(-1)
    })

    it('lowerBound and upperBound behave correctly', () => {
        const arr = [1, 2, 2, 2, 3, 5]
        expect(lowerBound(arr, 2)).toBe(1)
        expect(upperBound(arr, 2)).toBe(4)
        expect(lowerBound(arr, 4)).toBe(5)
        expect(upperBound(arr, 4)).toBe(5)
        expect(lowerBound(arr, -10)).toBe(0)
        expect(upperBound(arr, 100)).toBe(6)
    })

    it('works with empty arrays', () => {
        expect(binarySearch([], 1)).toBe(-1)
        expect(lowerBound([], 1)).toBe(0)
        expect(upperBound([], 1)).toBe(0)
        expect(linearSearch([], 1)).toBe(-1)
    })
})


