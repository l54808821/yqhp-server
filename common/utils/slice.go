package utils

import (
	"github.com/duke-git/lancet/v2/slice"
)

// Contains 判断切片是否包含元素
func SliceContains[T comparable](s []T, item T) bool {
	return slice.Contain(s, item)
}

// Unique 切片去重
func SliceUnique[T comparable](s []T) []T {
	return slice.Unique(s)
}

// Filter 过滤切片
func SliceFilter[T any](s []T, fn func(index int, item T) bool) []T {
	return slice.Filter(s, fn)
}

// Map 映射切片
func SliceMap[T any, U any](s []T, fn func(index int, item T) U) []U {
	return slice.Map(s, fn)
}

// Find 查找切片元素
func SliceFind[T any](s []T, fn func(index int, item T) bool) (*T, bool) {
	v, ok := slice.FindBy(s, fn)
	if !ok {
		return nil, false
	}
	return &v, true
}

// Reduce 归约切片
func SliceReduce[T any, U any](s []T, initial U, fn func(index int, item T, agg U) U) U {
	return slice.ReduceBy(s, initial, fn)
}

// Reverse 反转切片
func SliceReverse[T any](s []T) []T {
	slice.Reverse(s)
	return s
}

// Shuffle 打乱切片
func SliceShuffle[T any](s []T) []T {
	return slice.Shuffle(s)
}

// Chunk 切片分块
func SliceChunk[T any](s []T, size int) [][]T {
	return slice.Chunk(s, size)
}

// Difference 切片差集
func SliceDifference[T comparable](s1, s2 []T) []T {
	return slice.Difference(s1, s2)
}

// Intersection 切片交集
func SliceIntersection[T comparable](slices ...[]T) []T {
	return slice.Intersection(slices...)
}

// Union 切片并集
func SliceUnion[T comparable](slices ...[]T) []T {
	return slice.Union(slices...)
}

