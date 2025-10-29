export function linearSearch<T>(arr: ReadonlyArray<T>, target: T, equals: (a: T, b: T) => boolean = (a, b) => a === b): number {
    for (let i = 0; i < arr.length; i++) if (equals(arr[i], target)) return i
    return -1
}

export function binarySearch(arr: ReadonlyArray<number>, target: number): number {
    let lo = 0, hi = arr.length - 1
    while (lo <= hi) {
        const mid = lo + Math.floor((hi - lo) / 2)
        const v = arr[mid]
        if (v === target) return mid
        if (v < target) lo = mid + 1
        else hi = mid - 1
    }
    return -1
}

export function lowerBound(arr: ReadonlyArray<number>, target: number): number {
    let lo = 0, hi = arr.length
    while (lo < hi) {
        const mid = lo + Math.floor((hi - lo) / 2)
        if (arr[mid] < target) lo = mid + 1
        else hi = mid
    }
    return lo
}

export function upperBound(arr: ReadonlyArray<number>, target: number): number {
    let lo = 0, hi = arr.length
    while (lo < hi) {
        const mid = lo + Math.floor((hi - lo) / 2)
        if (arr[mid] <= target) lo = mid + 1
        else hi = mid
    }
    return lo
}


