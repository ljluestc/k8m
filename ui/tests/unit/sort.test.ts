import { bubbleSort, heapSort, insertionSort, mergeSort, quickSort, selectionSort, isStableSort } from '@/utils/algorithms/sort'

const algorithms = [
    ['quick', quickSort],
    ['merge', mergeSort],
    ['heap', heapSort],
    ['insertion', insertionSort],
    ['selection', selectionSort],
    ['bubble', bubbleSort],
] as const

function isSorted<T>(arr: T[], cmp: (a: T, b: T) => number): boolean {
    for (let i = 1; i < arr.length; i++) if (cmp(arr[i - 1], arr[i]) > 0) return false
    return true
}

describe('Sorting algorithms', () => {
    const cmp = (a: number, b: number) => a - b
    const cases: Array<number[]> = [
        [],
        [1],
        [1, 1, 1],
        [2, 1],
        [3, 2, 1],
        [1, 2, 3],
        [5, -1, 4, 0, 0, 3, 2, 2, 1],
        Array.from({ length: 50 }, (_, i) => i),
        Array.from({ length: 50 }, (_, i) => 49 - i),
        Array.from({ length: 100 }, () => Math.floor(Math.random() * 100) - 50),
    ]

    test.each(algorithms)('%s: basic correctness and properties', (_name, alg) => {
        for (const input of cases) {
            const out = alg(input, cmp)
            expect(out).toHaveLength(input.length)
            expect(isSorted(out, cmp)).toBe(true)
            // permutation check
            const a = [...input].sort(cmp)
            expect(out).toEqual(a)
            // idempotence
            expect(alg(out, cmp)).toEqual(out)
        }
    })

    it('supports custom comparator (desc)', () => {
        const desc = (a: number, b: number) => b - a
        for (const [, alg] of algorithms) {
            const out = alg([1, 2, 3, 4, 5], desc)
            expect(out).toEqual([5, 4, 3, 2, 1])
        }
    })

    it('stability: merge/insertion/bubble are stable; quick/heap/selection may be unstable', () => {
        const data = [
            { k: 1, id: 'a' },
            { k: 1, id: 'b' },
            { k: 2, id: 'c' },
            { k: 2, id: 'd' },
        ]
        const cmpObj = (a: { k: number }, b: { k: number }) => a.k - b.k

        expect(isStableSort(mergeSort, data, cmpObj)).toBe(true)
        expect(isStableSort(insertionSort, data, cmpObj)).toBe(true)
        expect(isStableSort(bubbleSort, data, cmpObj)).toBe(true)
    })
})


