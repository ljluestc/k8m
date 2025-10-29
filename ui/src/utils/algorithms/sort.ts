export type Comparator<T> = (a: T, b: T) => number

function defaultComparator<T>(a: T, b: T): number {
    // numeric or string fallback
    // @ts-expect-error
    if (a < b) return -1
    // @ts-expect-error
    if (a > b) return 1
    return 0
}

export function quickSort<T>(input: ReadonlyArray<T>, comparator: Comparator<T> = defaultComparator): T[] {
    const arr = [...input]
    function qs(lo: number, hi: number): void {
        if (lo >= hi) return
        const pivotIndex = partition(lo, hi)
        qs(lo, pivotIndex - 1)
        qs(pivotIndex + 1, hi)
    }
    function partition(lo: number, hi: number): number {
        const pivot = arr[hi]
        let i = lo
        for (let j = lo; j < hi; j++) {
            if (comparator(arr[j], pivot) <= 0) {
                ;[arr[i], arr[j]] = [arr[j], arr[i]]
                i++
            }
        }
        ;[arr[i], arr[hi]] = [arr[hi], arr[i]]
        return i
    }
    if (arr.length > 1) qs(0, arr.length - 1)
    return arr
}

export function mergeSort<T>(input: ReadonlyArray<T>, comparator: Comparator<T> = defaultComparator): T[] {
    const arr = [...input]
    if (arr.length <= 1) return arr
    const mid = Math.floor(arr.length / 2)
    const left = mergeSort(arr.slice(0, mid), comparator)
    const right = mergeSort(arr.slice(mid), comparator)
    return merge(left, right, comparator)
}

function merge<T>(a: T[], b: T[], comparator: Comparator<T>): T[] {
    const result: T[] = []
    let i = 0, j = 0
    while (i < a.length && j < b.length) {
        if (comparator(a[i], b[j]) <= 0) {
            result.push(a[i++])
        } else {
            result.push(b[j++])
        }
    }
    while (i < a.length) result.push(a[i++])
    while (j < b.length) result.push(b[j++])
    return result
}

export function heapSort<T>(input: ReadonlyArray<T>, comparator: Comparator<T> = defaultComparator): T[] {
    const arr = [...input]
    const n = arr.length
    function heapify(n: number, i: number): void {
        let largest = i
        const l = 2 * i + 1
        const r = 2 * i + 2
        if (l < n && comparator(arr[l], arr[largest]) > 0) largest = l
        if (r < n && comparator(arr[r], arr[largest]) > 0) largest = r
        if (largest !== i) {
            ;[arr[i], arr[largest]] = [arr[largest], arr[i]]
            heapify(n, largest)
        }
    }
    for (let i = Math.floor(n / 2) - 1; i >= 0; i--) heapify(n, i)
    for (let i = n - 1; i > 0; i--) {
        ;[arr[0], arr[i]] = [arr[i], arr[0]]
        heapify(i, 0)
    }
    return arr
}

export function insertionSort<T>(input: ReadonlyArray<T>, comparator: Comparator<T> = defaultComparator): T[] {
    const arr = [...input]
    for (let i = 1; i < arr.length; i++) {
        const key = arr[i]
        let j = i - 1
        while (j >= 0 && comparator(arr[j], key) > 0) {
            arr[j + 1] = arr[j]
            j--
        }
        arr[j + 1] = key
    }
    return arr
}

export function selectionSort<T>(input: ReadonlyArray<T>, comparator: Comparator<T> = defaultComparator): T[] {
    const arr = [...input]
    for (let i = 0; i < arr.length; i++) {
        let minIndex = i
        for (let j = i + 1; j < arr.length; j++) {
            if (comparator(arr[j], arr[minIndex]) < 0) minIndex = j
        }
        if (minIndex !== i) {
            ;[arr[i], arr[minIndex]] = [arr[minIndex], arr[i]]
        }
    }
    return arr
}

export function bubbleSort<T>(input: ReadonlyArray<T>, comparator: Comparator<T> = defaultComparator): T[] {
    const arr = [...input]
    const n = arr.length
    let swapped = true
    for (let i = 0; i < n - 1 && swapped; i++) {
        swapped = false
        for (let j = 0; j < n - i - 1; j++) {
            if (comparator(arr[j], arr[j + 1]) > 0) {
                ;[arr[j], arr[j + 1]] = [arr[j + 1], arr[j]]
                swapped = true
            }
        }
    }
    return arr
}

export function isStableSort<T>(algorithm: (arr: ReadonlyArray<T>, cmp?: Comparator<T>) => T[], data: ReadonlyArray<T>, cmp: Comparator<T>): boolean {
    // Compare positions of equal keys
    const withIndex = data.map((v, i) => ({ v, i }))
    const sorted = algorithm(withIndex, (a, b) => {
        const res = cmp(a.v, b.v)
        return res !== 0 ? res : a.i - b.i
    })
    for (let i = 1; i < sorted.length; i++) {
        if (cmp(sorted[i - 1].v, sorted[i].v) === 0 && sorted[i - 1].i > sorted[i].i) return false
    }
    return true
}


